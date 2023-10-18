// Copyright 2023 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/reexec"
)

func runSelf(args ...string) ([]byte, error) {
	cmd := &exec.Cmd{
		Path: reexec.Self(),
		Args: append([]string{"geth-test"}, args...),
	}
	return cmd.CombinedOutput()
}

func split(input io.Reader) []string {
	var output []string
	scanner := bufio.NewScanner(input)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		output = append(output, scanner.Text())
	}
	return output
}

func censor(input string, start, end int) string {
	if len(input) < end {
		return input
	}
	return input[:start] + strings.Repeat("X", end-start) + input[end:]
}

func TestLogging(t *testing.T) {
	testConsoleLogging(t, "terminal", 6, 24)
	testConsoleLogging(t, "logfmt", 2, 26)
}

func testConsoleLogging(t *testing.T, format string, tStart, tEnd int) {
	haveB, err := runSelf("--log.format", format, "logtest")
	if err != nil {
		t.Fatal(err)
	}
	readFile, err := os.Open(fmt.Sprintf("testdata/logging/logtest-%v.txt", format))
	if err != nil {
		t.Fatal(err)
	}
	wantLines := split(readFile)
	haveLines := split(bytes.NewBuffer(haveB))
	for i, want := range wantLines {
		if i > len(haveLines)-1 {
			t.Fatalf("format %v, line %d missing, want:%v", format, i, want)
		}
		have := haveLines[i]
		// Black out the timestamp
		have = censor(have, tStart, tEnd)
		want = censor(want, tStart, tEnd)
		if have != want {
			t.Fatalf("format %v, line %d\nhave %v\nwant %v", format, i, have, want)
		}
	}
	if len(haveLines) != len(wantLines) {
		t.Errorf("format %v, want %d lines, have %d", format, len(haveLines), len(wantLines))
	}
}
