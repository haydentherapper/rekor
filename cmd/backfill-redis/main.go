// Copyright 2022 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"

	"github.com/go-openapi/runtime"
	radix "github.com/mediocregopher/radix/v4"
	"github.com/sigstore/rekor/pkg/client"
	"github.com/sigstore/rekor/pkg/generated/client/entries"
	"github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sigstore/rekor/pkg/types"

	// these imports are to call the packages' init methods
	_ "github.com/sigstore/rekor/pkg/types/alpine/v0.0.1"
	_ "github.com/sigstore/rekor/pkg/types/cose/v0.0.1"
	_ "github.com/sigstore/rekor/pkg/types/hashedrekord/v0.0.1"
	_ "github.com/sigstore/rekor/pkg/types/helm/v0.0.1"
	_ "github.com/sigstore/rekor/pkg/types/intoto/v0.0.1"
	_ "github.com/sigstore/rekor/pkg/types/intoto/v0.0.2"
	_ "github.com/sigstore/rekor/pkg/types/jar/v0.0.1"
	_ "github.com/sigstore/rekor/pkg/types/rekord/v0.0.1"
	_ "github.com/sigstore/rekor/pkg/types/rfc3161/v0.0.1"
	_ "github.com/sigstore/rekor/pkg/types/rpm/v0.0.1"
	_ "github.com/sigstore/rekor/pkg/types/tuf/v0.0.1"
)

var (
	redisAddress = flag.String("address", "", "Address for Redis application")
	redisPort    = flag.String("port", "", "Port to Redis application")
	startIndex   = flag.Int("start", -1, "First index to backfill")
	endIndex     = flag.Int("end", -1, "Last index to backfill")
	rekorAddress = flag.String("rekor-address", "", "Address for Rekor, e.g. https://rekor.sigstore.dev")
)

func main() {
	flag.Parse()

	if *redisAddress == "" {
		log.Fatal("address must be set")
	}
	if *redisPort == "" {
		log.Fatal("port must be set")
	}
	if *startIndex == -1 {
		log.Fatal("start must be set to >=0")
	}
	if *endIndex == -1 {
		log.Fatal("end must be set to >=0")
	}
	if *rekorAddress == "" {
		log.Fatal("rekor-address must be set")
	}

	cfg := radix.PoolConfig{}
	redisClient, err := cfg.New(context.Background(), "tcp", fmt.Sprintf("%s:%s", *redisAddress, *redisPort))
	if err != nil {
		log.Fatal(err)
	}

	rekorClient, err := client.GetRekorClient(*rekorAddress)
	if err != nil {
		log.Fatalf("creating rekor client: %v", err)
	}

	for i := *startIndex; i <= *endIndex; i++ {
		params := entries.NewGetLogEntryByIndexParamsWithContext(context.Background())
		params.SetLogIndex(int64(i))
		resp, err := rekorClient.Entries.GetLogEntryByIndex(params)
		if err != nil {
			log.Fatalf("retrieving log uuid by index: %v", err)
		}
		success := true
		for uuid, entry := range resp.Payload {
			// uuid is the global UUID - tree ID and entry UUID
			e, _, _, err := unmarshalEntryImpl(entry.Body.(string))
			if err != nil {
				fmt.Printf("error unmarshalling entry for %s: %v\n", uuid, err)
				success = false
				continue
			}
			keys, err := e.IndexKeys()
			if err != nil {
				fmt.Printf("error building index keys for %s: %v\n", uuid, err)
				success = false
				continue
			}
			for _, key := range keys {
				if err := addToIndex(context.Background(), redisClient, key, uuid); err != nil {
					success = false
					fmt.Printf("error inserting UUID %s with key %s: %v\n", uuid, key, err)
				}
				fmt.Printf("Uploaded Redis entry %s, index %d, key %s\n", uuid, i, key)
			}
		}
		if success {
			fmt.Printf("Completed log index %d\n", i)
		} else {
			fmt.Printf("Errors with log index %d\n", i)
		}
	}
}

// unmarshalEntryImpl decodes the base64-encoded entry to a specific entry type (types.EntryImpl).
// from cosign
func unmarshalEntryImpl(e string) (types.EntryImpl, string, string, error) {
	b, err := base64.StdEncoding.DecodeString(e)
	if err != nil {
		return nil, "", "", err
	}

	pe, err := models.UnmarshalProposedEntry(bytes.NewReader(b), runtime.JSONConsumer())
	if err != nil {
		return nil, "", "", err
	}

	entry, err := types.UnmarshalEntry(pe)
	if err != nil {
		return nil, "", "", err
	}
	return entry, pe.Kind(), entry.APIVersion(), nil
}

func addToIndex(ctx context.Context, redisClient radix.Client, key, value string) error {
	return redisClient.Do(ctx, radix.Cmd(nil, "LPUSH", key, value))
}
