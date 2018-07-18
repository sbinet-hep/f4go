package fortran

import (
	"fmt"
	"go/token"
)

const (
	DOUBLE_STAR token.Token = iota + token.VAR + 10 // **
	SUBROUTINE
	PROGRAM
	INTEGER
	DOUBLE_COLON
	IMPLICIT
	FUNCTION
	END
	DO
	ENDDO
	CALL
	THEN
	NEW_LINE
)

func view(t token.Token) string {
	if int(t) < int(token.VAR)+1 {
		return fmt.Sprintf("%s", t)
	}
	return o[t]
}

var o = [...]string{
	SUBROUTINE:   "SUBROUTINE",
	PROGRAM:      "PROGRAM",
	INTEGER:      "INTEGER",
	DOUBLE_COLON: "DOUBLE_COLON",
	IMPLICIT:     "IMPLICIT",
	FUNCTION:     "FUNCTION",
	END:          "END",
	DO:           "DO",
	ENDDO:        "ENDDO",
	CALL:         "CALL",
	THEN:         "THEN",
	NEW_LINE:     "NEW_LINE",
}
