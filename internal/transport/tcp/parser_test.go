package tcp

import (
	"bytes"
	"fmt"
	"testing"
)

type TestCase struct {
	Name      string
	Input     string
	Want      Command
	ExpectErr bool
}

func compareCommands(got, want Command) error {
	if got.Type != want.Type {
		return fmt.Errorf("Type mismatch: got %v, want %v", got.Type, want.Type)
	}
	if got.Key != want.Key {
		return fmt.Errorf("Key mismatch: got %q, want %q", got.Key, want.Key)
	}
	if !bytes.Equal(got.Value, want.Value) {
		return fmt.Errorf("Value mismatch: got %q, want %q", string(got.Value), string(want.Value))
	}

	// Pointer comparison logic for TTL
	if (got.TTL == nil) != (want.TTL == nil) {
		return fmt.Errorf("TTL presence mismatch")
	}
	if got.TTL != nil && want.TTL != nil {
		if *got.TTL != *want.TTL {
			return fmt.Errorf("TTL value mismatch: got %d, want %d", *got.TTL, *want.TTL)
		}
	}
	return nil
}

func createTestCases() []TestCase {
	ptr := func(v int64) *int64 { return &v }

	tests := []TestCase{
		{
			Name:  "SET standard",
			Input: `SET "user" VALUE "john"`,
			Want:  Command{Type: CmdSet, Key: "user", Value: []byte("john")},
		},
		{
			Name:  "SET with TTL",
			Input: `SET "user" VALUE "john" TTL "60"`,
			Want:  Command{Type: CmdSet, Key: "user", Value: []byte("john"), TTL: ptr(60)},
		},
		{
			Name:  "SET with escaped JSON",
			Input: `SET "user" VALUE "{\"name\":\"john\"}"`,
			Want:  Command{Type: CmdSet, Key: "user", Value: []byte(`{"name":"john"}`)},
		},
		{
			Name:      "SET with invalid TTL alpha",
			Input:     `SET "user" VALUE "john" TTL "abc"`,
			ExpectErr: true,
		},
		{
			Name:      "SET missing quotes on key",
			Input:     `SET user VALUE "john"`,
			ExpectErr: true,
		},
		{
			Name:  "GET standard",
			Input: `GET "user"`,
			Want:  Command{Type: CmdGet, Key: "user"},
		},
		{
			Name:  "GET with lowercase",
			Input: `get "user"`,
			Want:  Command{Type: CmdGet, Key: "user"},
		},
		{
			Name:  "DELETE standard",
			Input: `DELETE "user"`,
			Want:  Command{Type: CmdDelete, Key: "user"},
		},
		{
			Name:  "CLEAR standard",
			Input: `CLEAR`,
			Want:  Command{Type: CmdClear},
		},
		{
			Name:      "CLEAR with invalid extra args",
			Input:     `CLEAR "extra"`,
			ExpectErr: true,
		},
		{
			Name:  "SET with escaped internal quotes",
			Input: `SET "key" VALUE "value with \"quotes\""`,
			Want:  Command{Type: CmdSet, Key: "key", Value: []byte(`value with "quotes"`)},
		},
		{
			Name:  "SET with zero TTL",
			Input: `SET "k" VALUE "v" TTL "0"`,
			Want:  Command{Type: CmdSet, Key: "k", Value: []byte("v"), TTL: ptr(0)},
		},
		{
			Name:  "SET with negative TTL",
			Input: `SET "k" VALUE "v" TTL "-10"`,
			Want:  Command{Type: CmdSet, Key: "k", Value: []byte("v"), TTL: ptr(-10)},
		},
		{
			Name:      "SET with TTL overflow",
			Input:     `SET "k" VALUE "v" TTL "999999999999999999999"`,
			ExpectErr: true,
		},
		{
			Name:      "SET with quoted quote as TTL",
			Input:     `SET "k" VALUE "v" TTL "\""`,
			ExpectErr: true,
		},
		{
			Name:  "SET with empty string value",
			Input: `SET "k" VALUE ""`,
			Want:  Command{Type: CmdSet, Key: "k", Value: []byte("")},
		},
		{
			Name:      "SET with swapped keywords",
			Input:     `SET "k" "v" VALUE`,
			ExpectErr: true,
		},
		{
			Name:      "SET with TTL before VALUE",
			Input:     `SET "k" TTL "10" VALUE "v"`,
			ExpectErr: true,
		},
		{
			Name:      "SET with unknown trailing keyword",
			Input:     `SET "k" VALUE "v" EXTRA "x"`,
			ExpectErr: true,
		},
		{
			Name:  "SET with extra whitespace",
			Input: `SET    "user"   VALUE    "john"`,
			Want:  Command{Type: CmdSet, Key: "user", Value: []byte("john")},
		},
		{
			Name:  "SET with unicode/emojis",
			Input: `SET "🔑" VALUE "🔥"`,
			Want:  Command{Type: CmdSet, Key: "🔑", Value: []byte("🔥")},
		},
		{
			Name:  "SET with tab characters",
			Input: "SET\t\"k\"\tVALUE\t\"v\"",
			Want:  Command{Type: CmdSet, Key: "k", Value: []byte("v")},
		},
		{
			Name:      "SET with multi-line JSON",
			Input:     `SET "config" VALUE "{\n  \"enabled\": true\n}"`,
			ExpectErr: true,
		},
		{
			Name:      "SET with missing value quotes",
			Input:     `SET "key" VALUE value`,
			ExpectErr: true,
		},
	}

	return tests
}

func TestParse(t *testing.T) {
	tests := createTestCases()

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := Parse(tt.Input)

			if (err != nil) != tt.ExpectErr {
				t.Fatalf("Parse(%s) unexpected error state: %v", tt.Input, err)
			}

			if !tt.ExpectErr {
				if diff := compareCommands(got, tt.Want); diff != nil {
					t.Errorf("\nInput: %s\nGot:  %+v\nWant: %+v", tt.Input, got, tt.Want)
				}
			}
		})
	}
}
