package clilog

import (
	"fmt"
	"strings"
)

type CliOut struct {
	Prefix      string
	Quiet       bool
	indentation int
}

func (l *CliOut) AddIndent() {
	l.indentation++
}
func (l *CliOut) RemIndent() {
	l.indentation--
	if l.indentation < 0 {
		l.indentation = 0
	}
}

func (l *CliOut) Print(msg string) {
	if l.Quiet {
		return
	}

	if l.indentation == 0 {
		fmt.Println(l.Prefix + msg)
	} else {
		fmt.Println(strings.Repeat("  ", l.indentation) + strings.Repeat(" ", len(l.Prefix)) + msg)
	}
}
