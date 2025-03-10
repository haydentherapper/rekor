//
// Copyright 2021 The Sigstore Authors.
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

package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sigstore/rekor/pkg/api"
)

type LogRangesFlag struct {
	Ranges api.LogRanges
}

func (l *LogRangesFlag) Set(s string) error {
	ranges := strings.Split(s, ",")
	l.Ranges = api.LogRanges{}

	var err error
	inputRanges := []api.LogRange{}

	// Only go up to the second to last one, the last one is special cased beow
	for _, r := range ranges[:len(ranges)-1] {
		split := strings.SplitN(r, "=", 2)
		if len(split) != 2 {
			return fmt.Errorf("invalid range flag, expected two parts separated by an =, got %s", r)
		}
		lr := api.LogRange{}
		lr.TreeID, err = strconv.ParseUint(split[0], 10, 64)
		if err != nil {
			return err
		}
		lr.TreeLength, err = strconv.ParseUint(split[1], 10, 64)
		if err != nil {
			return err
		}

		inputRanges = append(inputRanges, lr)
	}

	// The last entry is special and should not have a terminating range, because this is active.
	lastRangeStr := ranges[len(ranges)-1]
	lastTreeID, err := strconv.ParseUint(lastRangeStr, 10, 64)
	if err != nil {
		return err
	}

	inputRanges = append(inputRanges, api.LogRange{
		TreeID: lastTreeID,
	})

	// Look for duplicate tree ids
	TreeIDs := map[uint64]struct{}{}
	for _, lr := range inputRanges {
		if _, ok := TreeIDs[lr.TreeID]; ok {
			return fmt.Errorf("duplicate tree id: %d", lr.TreeID)
		}
		TreeIDs[lr.TreeID] = struct{}{}
	}

	l.Ranges = api.LogRanges{
		Ranges: inputRanges,
	}
	return nil
}

func (l *LogRangesFlag) String() string {
	ranges := []string{}
	for _, r := range l.Ranges.Ranges {
		ranges = append(ranges, fmt.Sprintf("%d=%d", r.TreeID, r.TreeLength))
	}
	return strings.Join(ranges, ",")
}

func (l *LogRangesFlag) Type() string {
	return "LogRangesFlag"
}
