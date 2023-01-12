/*
	Copyright <2022> Nik Ogura <nik.ogura@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package boilerplate

import (
	"bufio"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"os"
	"strings"
)

type Prompt struct {
	From         io.Reader
	PromptMsg    string
	Desc         string
	InputFailMsg string
	DefaultValue string
	Validations  []PromptValidation
}

type PromptValidation struct {
	IsValid    func(val string) bool
	InvalidMsg string
}

func PromptForInput(p Prompt) (data string, err error) {
	if p.From == nil {
		p.From = os.Stdin
	}

	reader := bufio.NewReader(p.From)
	var msg string
	if p.DefaultValue != "" {
		msg = fmt.Sprintf("%s [default: %s]:", p.PromptMsg, p.DefaultValue)
	} else {
		msg = fmt.Sprintf("%s:", p.PromptMsg)
	}
	msg = fmt.Sprintf("%s\n  value: ", msg)

	fmt.Printf(msg)

	input, err := reader.ReadString('\n')
	if err != nil {
		msg := "failed to read input name"
		if p.InputFailMsg != "" {
			msg = p.InputFailMsg
		}
		err = errors.Wrapf(err, msg)
		return data, err
	}

	data = strings.TrimRight(input, "\n")

	if data == "" && p.DefaultValue != "" {
		data = p.DefaultValue
	}

	for _, v := range p.Validations {
		if !v.IsValid(data) {
			return data, fmt.Errorf("%s input: %q", v.InvalidMsg, data)
		}
	}

	return data, err
}
