package parser

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/golang-collections/collections/stack"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

type token int

const (
	unknown token = iota

	// root represents the opening position of the file outside of
	// any sections
	root

	// section represents a named section of data
	section

	// opener represents the opening token of a section
	opener

	// closer represents the closing token of a section
	closer

	// data represents a line of data
	data

	// openData represents a line of data that hasn't been concluded
	// (spills over to another line)
	openData

	// empty lines (e.g. comments or whitespace) are to be ignored
	empty
)

const (
	whitespaceCutset = " \t"
)

var (
	language = map[token]map[token]interface{}{
		root: {
			section: struct{}{},
		},
		section: {
			opener: struct{}{},
		},
		opener: {
			section:  struct{}{},
			data:     struct{}{},
			openData: struct{}{},
			closer:   struct{}{},
		},
		closer: {
			section:  struct{}{},
			data:     struct{}{},
			openData: struct{}{},
			closer:   struct{}{},
		},
		data: {
			section:  struct{}{},
			data:     struct{}{},
			openData: struct{}{},
			closer:   struct{}{},
		},
		openData: {
			data:     struct{}{},
			openData: struct{}{},
		},
	}
)

func resetFileStart(file *os.File) {
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		panic(err)
	}
}

func isUTF16(file *os.File) (bool, error) {
	// Read the first few bytes of the file to check for the Byte Order Mark (BOM)
	buf := make([]byte, 3)
	_, err := file.Read(buf)
	if err != nil {
		defer resetFileStart(file)
		if err == io.EOF {
			return false, nil
		}
		return false, err
	}

	// UTF-8 BOM checks first (U+FEFF). See https://symbl.cc/en/FEFF/
	if buf[0] == 0xef && buf[1] == 0xbb && buf[2] == 0xbf {
		return false, nil
	}
	// Corresponds to the U+FEFF unicode character in decimal. This is a hack.
	if buf[0] == 239 && buf[1] == 187 && buf[2] == 191 {
		return false, nil
	}

	// We only reset the file pointer if we are sure that the file is not UTF-8.
	// Not skipping the UTF-8 BOM causes issues with parsing.
	defer resetFileStart(file)

	// Check if the bytes match the UTF-16 BOM
	if buf[0] == 0xff && buf[1] == 0xfe {
		return true, nil
	}
	if buf[0] == 0xfe && buf[1] == 0xff {
		return true, nil
	}

	// The file does not have a BOM, so it is not UTF-16
	return false, nil
}

func convertFileToUTF8(file *os.File) (*bytes.Buffer, error) {
	fmt.Println("The file is encoded in UTF-16. We will try to convert it to UTF-8 in memory.")
	utf16Reader := bufio.NewReader(file)
	//bom := unicode.BOMOverride(utf16Reader)
	// Right now we assume that the file is encoded in UTF-16 Little Endian.
	// Since csgo_english.txt is encoded in UTF-16LE when copied from the game files.
	utf16Decoder := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()

	utf8Reader := transform.NewReader(utf16Reader, utf16Decoder)

	var buf bytes.Buffer
	utf8Writer := bufio.NewWriter(&buf)
	_, err := io.Copy(utf8Writer, utf8Reader)
	if err != nil {
		return nil, err
	}

	err = utf8Writer.Flush()
	if err != nil {
		return nil, err
	}

	return &buf, nil
}

// Maybe removalable
func isWhitespace(line string) bool {
	// The new version of csgo_english contains some extra empty lines, however these can have varying amounts of length.
	// So we will use a regex to check if the line is empty (or only contains whitespace).
	// Note: This is a hack, but U+FEFF is terrible.

	// Create a regular expression pattern to match whitespace or U+FEFF (BOM)

	pattern := regexp.MustCompile(`^(|\s|\n|\xEF\xBB\xBF)*$`)

	// Use the MatchString function to check if the line matches the pattern
	return pattern.MatchString(line)
}

func Parse(fileLocation string) (map[string]interface{}, error) {

	// initialise/reset
	dataTree := make(map[string]interface{})
	openSections := stack.New()
	openSections.Push(dataTree)

	lastToken := root

	fi, err := os.Open(fileLocation)
	if err != nil {
		return nil, err
	}
	defer fi.Close()

	fmt.Println("Checking if the file is encoded in UTF-16.")
	isUTF16File, err := isUTF16(fi)
	if err != nil {
		return nil, err
	}

	var s *bufio.Scanner
	if isUTF16File {
		buf, err := convertFileToUTF8(fi)
		if err != nil {
			return nil, err
		}
		fmt.Println("Successfully converted the file to UTF-8!")

		reader := bytes.NewReader(buf.Bytes())
		s = bufio.NewScanner(reader)
	} else {
		fmt.Println("The file is encoded in UTF-8.")
		s = bufio.NewScanner(fi)
	}

	lineCount := 0
	currentLine := ""

	// process lines
	for s.Scan() {

		lineCount++

		if s.Err() != nil {
			return nil, fmt.Errorf("failed to scan line %d with error: %s", lineCount, s.Err())
		}

		if lastToken == openData {
			currentLine += s.Text()
		} else {
			currentLine = s.Text()
		}

		t, lineData, err := getLineType(currentLine)
		if err != nil {
			fmt.Println(s.Text())
			return nil, fmt.Errorf("unable to parse line %d with error: %s", lineCount, err.Error())
		}

		// ignore empty lines
		if t == empty {
			currentLine = ""
			continue
		}

		// if token is unexpected
		if _, ok := language[lastToken][t]; !ok {
			return nil, fmt.Errorf("unexpected token on line %d", lineCount)
		}

		lastToken = t

		// process token
		switch t {

		// section: add new section to current section and add to stack
		case section:
			currentSection := openSections.Peek().(map[string]interface{})
			subsection := lineData[0]

			// if subsection already exists in currentSection
			if m, ok := currentSection[subsection]; ok {
				openSections.Push(m)
				continue
			}

			// else add it and push onto stack
			currentSection[subsection] = make(map[string]interface{})
			openSections.Push(currentSection[subsection])
			continue

		// closer: close currently open section
		case closer:
			if openSections.Len() == 0 {
				return nil, errors.New("unexpected token")
			}

			openSections.Pop()
			continue

		// data: add data to currently open section
		case data:
			currentSection := openSections.Peek().(map[string]interface{})
			currentSection[lineData[0]] = lineData[1]
			continue

		// opener, empty: ignore line as doesn't contain data
		case opener, openData, empty:
			continue
		}
	}

	return dataTree, nil
}

func getLineType(line string) (token, []string, error) {

	line = strings.Trim(line, whitespaceCutset)

	// if line is 0 chars after trim (or is a comment), it is a blank line to ignore
	if len(line) == 0 || strings.HasPrefix(line, "//") || isWhitespace(line) {
		return empty, nil, nil
	}

	// opener/closer is a single { or } (respectively)
	if len(line) == 1 {

		if line == "{" {
			return opener, nil, nil
		}

		if line == "}" {
			return closer, nil, nil
		}
	}

	split, open := parseDataLine(line)
	if open {
		return openData, nil, nil
	}

	if len(split) == 1 {
		return section, split, nil
	}

	if len(split) == 2 {
		return data, split, nil
	}

	return unknown, nil, errors.New("unrecognised line type")
}

// parseLineData returns the string sub-elements of a data line as a slice
// of strings, and a boolean to highlight whether the line is open ended.
func parseDataLine(line string) ([]string, bool) {

	subStrings := make([]string, 0)
	currentString := ""

	quoted := false

	for i, c := range line {

		// break if string is comment (comments begin with '//')
		if !quoted && c == '/' {
			if i < len(line)-1 {
				if line[i+1] == '/' {
					break
				}
			}
		}

		// if we reach a quote mark
		if c == '"' {

			// and the quote isn't escaped
			if i == 0 || line[i-1] != '\\' {

				// add current string if necessary and reset
				if quoted {
					subStrings = append(subStrings, currentString)
					currentString = ""
				}

				// flip quoted
				quoted = !quoted
				continue
			}
		}

		// ignore anything outside of quotes
		if !quoted {
			continue
		}

		currentString += string(c)
	}

	return subStrings, quoted
}
