package tcp

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type CommandType int

const (
	CmdSet CommandType = iota
	CmdGet
	CmdDelete
	CmdClear
)

type Command struct {
	Type  CommandType
	Key   string
	Value []byte
	TTL   *int64
}

type token struct {
	value  string
	quoted bool
}

type ParseError struct {
	Message string
}

func (parseError *ParseError) Error() string {
	return parseError.Message
}

func Parse(input string) (Command, error) {
	tokens, err := tokenize(input)

	if err != nil {
		return Command{}, err
	}

	if len(tokens) == 0 {
		return Command{}, &ParseError{Message: "empty command"}
	}

	switch tokens[0].value {
	case "SET":
		return parseSet(tokens)
	case "GET":
		return parseGet(tokens)
	case "DELETE":
		return parseDelete(tokens)
	case "CLEAR":
		return parseClear(tokens)
	default:
		return Command{}, &ParseError{
			Message: fmt.Sprintf("unknown command: %s", tokens[0].value),
		}
	}
}

func tokenize(input string) ([]token, error) {
	var tokens []token

	i := 0
	n := len(input)

	for i < n {
		// whitespace skipping
		for i < n && unicode.IsSpace(rune(input[i])) {
			i++
		}

		if i >= n {
			break
		}

		// quoted
		if input[i] == '"' {
			i++

			var builder strings.Builder
			closed := false

			for i < n {
				ch := input[i]

				if ch == '\\' {
					if i+1 >= n {
						return nil, &ParseError{
							Message: "invalid escape sequence",
						}
					}
					next := input[i+1]
					switch next {
					case '"', '\\':
						builder.WriteByte(next)
						i += 2
					default:
						return nil, &ParseError{
							Message: fmt.Sprintf("unsupported escape sequence: \\%c", next),
						}
					}
					continue
				}

				if ch == '"' {
					i++
					closed = true
					break
				}

				builder.WriteByte(ch)
				i++
			}

			if !closed {
				return nil, &ParseError{
					Message: "unterminated quoted string",
				}
			}

			tokens = append(tokens, token{
				value:  builder.String(),
				quoted: true,
			})
			continue
		}
		// unquoted (keywords)
		start := i
		for i < n && !unicode.IsSpace(rune(input[i])) {
			i++
		}
		tokens = append(tokens, token{
			value:  strings.ToUpper(input[start:i]),
			quoted: false,
		})
	}

	return tokens, nil
}

func parseSet(tokens []token) (Command, error) {
	// Expected:
	// SET "key" VALUE "value" [TTL "seconds"]

	if len(tokens) < 4 {
		return Command{}, &ParseError{
			Message: "SET requires at least 4 tokens",
		}
	}

	if !tokens[1].quoted {
		return Command{}, &ParseError{
			Message: "key must be quoted",
		}
	}

	if tokens[2].value != "VALUE" {
		return Command{}, &ParseError{
			Message: "expected VALUE keyword",
		}
	}

	if !tokens[3].quoted {
		return Command{}, &ParseError{
			Message: "value must be quoted",
		}
	}

	command := Command{
		Type:  CmdSet,
		Key:   tokens[1].value,
		Value: []byte(tokens[3].value),
	}

	// no TTL
	if len(tokens) == 4 {
		return command, nil
	}

	// yes TTL
	if len(tokens) != 6 {
		return Command{}, &ParseError{
			Message: "invalid SET syntax",
		}
	}

	if tokens[4].value != "TTL" {
		return Command{}, &ParseError{
			Message: "expected TTL keyword",
		}
	}

	if !tokens[5].quoted {
		return Command{}, &ParseError{
			Message: "TTL value must be quoted",
		}
	}

	ttl, err := strconv.ParseInt(tokens[5].value, 10, 64)
	if err != nil {
		return Command{}, &ParseError{
			Message: "invalid TTL value",
		}
	}

	command.TTL = &ttl
	return command, nil
}

func parseGet(tokens []token) (Command, error) {
	if len(tokens) != 2 {
		return Command{}, &ParseError{
			Message: "GET requires exactly 1 argument",
		}
	}

	if !tokens[1].quoted {
		return Command{}, &ParseError{
			Message: "key must be quoted",
		}
	}

	return Command{
		Type: CmdGet,
		Key:  tokens[1].value,
	}, nil
}

func parseDelete(tokens []token) (Command, error) {
	if len(tokens) != 2 {
		return Command{}, &ParseError{
			Message: "DELETE requires exactly 1 argument",
		}
	}

	if !tokens[1].quoted {
		return Command{}, &ParseError{
			Message: "key must be quoted",
		}
	}

	return Command{
		Type: CmdDelete,
		Key:  tokens[1].value,
	}, nil
}

func parseClear(tokens []token) (Command, error) {
	if len(tokens) != 1 {
		return Command{}, &ParseError{
			Message: "CLEAR takes no arguments",
		}
	}

	return Command{
		Type: CmdClear,
	}, nil
}

func (commandType CommandType) String() string {
	switch commandType {
	case CmdSet:
		return "SET"
	case CmdGet:
		return "GET"
	case CmdDelete:
		return "DELETE"
	case CmdClear:
		return "CLEAR"
	default:
		return "UNKNOWN"
	}
}
