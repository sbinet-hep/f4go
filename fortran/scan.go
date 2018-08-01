package fortran

import (
	"bytes"
	"container/list"
	"fmt"
	"go/token"
	"strings"
)

type position struct {
	line int // line
	col  int // column
}

type ele struct {
	tok token.Token
	b   []byte
	pos position
}

func (e ele) String() string {
	return fmt.Sprintf("[%v, `%s`, %v]", view(e.tok), string(e.b), e.pos)
}

func (e *ele) Split() (eles []ele) {
	var b []byte
	b = append(b, e.b...)

	var offset int
	for {
		if len(b) == 0 {
			break
		}

		var st int
		for st = 0; st < len(b); st++ {
			if b[st] != ' ' {
				break
			}
		}

		var end int
		for end = st; end < len(b) && b[end] != ' '; end++ {
		}

		if end-st == 0 {
			break
		} else {
			eles = append(eles, ele{
				tok: e.tok,
				pos: position{
					line: e.pos.line,
					col:  e.pos.col + st + offset,
				},
				b: b[st:end],
			})
		}
		if end >= len(b) {
			break
		}
		b = b[end:]
		offset += end
	}

	return
}

// scanner represents a lexical scanner.
type elScan struct {
	eles *list.List
}

// newScanner returns a new instance of Scanner.
func scanT(b []byte) *list.List {
	var s elScan
	s.eles = list.New()
	s.eles.PushFront(&ele{
		tok: undefine,
		b:   b,
		pos: position{
			line: 1,
			col:  1,
		},
	})

	// separate lines
	s.scanBreakLines()

	// separate comments
	s.scanComments()

	// separate strings
	s.scanStrings()

	// preprocessor: add specific spaces
	s.scanTokenWithPoint()
	defer func() {
		// postprocessor
		s.postprocessor()
	}()

	// separate on other token
	s.scanTokens()

	// remove empty
	s.scanEmpty()

	// scan numbers
	s.scanNumbers()

	// remove empty
	s.scanEmpty()

	s.scanTokensAfter()

	// remove empty
	s.scanEmpty()

	// IDENT for undefine
	for e := s.eles.Front(); e != nil; e = e.Next() {
		switch e.Value.(*ele).tok {
		case undefine:
			e.Value.(*ele).tok = token.IDENT
		}
	}

	// token GO TO
	s.scanGoto()

	return s.eles
}

// separate break lines
func (s *elScan) scanBreakLines() {
B:
	var again bool
	for e := s.eles.Front(); e != nil; e = e.Next() {
		switch e.Value.(*ele).tok {
		case NEW_LINE:
			// ignore
		default:
			for j := len(e.Value.(*ele).b) - 1; j >= 0; j-- {
				if e.Value.(*ele).b[j] != '\n' {
					continue
				}
				s.extract(j, j+1, e, NEW_LINE)
				again = true
			}
		}
	}
	if again {
		goto B
	}
	line := 1
	for e := s.eles.Front(); e != nil; e = e.Next() {
		e.Value.(*ele).pos.line = line
		e.Value.(*ele).pos.col = 1
		if e.Value.(*ele).tok == NEW_LINE {
			line++
		}
	}
}

// separate comments
func (s *elScan) scanComments() {
	// comments single line started from letters:
	// 'C', 'c', '*', 'd', 'D'
	for e := s.eles.Front(); e != nil; e = e.Next() {
		switch e.Value.(*ele).tok {
		case undefine:
			if len(e.Value.(*ele).b) == 0 {
				continue
			}
			ch := e.Value.(*ele).b[0]
			if ch == 'C' || ch == 'c' ||
				ch == '*' ||
				ch == 'D' || ch == 'd' {
				e.Value.(*ele).tok = token.COMMENT
			}
		}
	}

	// comments inside line : '!'
Op:
	var again bool
	for e := s.eles.Front(); e != nil; e = e.Next() {
		switch e.Value.(*ele).tok {
		case undefine:
			if len(e.Value.(*ele).b) == 0 {
				continue
			}
			var st int
			var found bool
			for st = 0; st < len(e.Value.(*ele).b); st++ {
				if e.Value.(*ele).b[st] == '!' {
					found = true
					break
				}
			}
			if found {
				s.extract(st, len(e.Value.(*ele).b), e, token.COMMENT)
				again = true
			}
		}
	}
	if again {
		goto Op
	}
}

// extract
// start - column started  (included)
// end   - column finished (not included)
func (s *elScan) extract(start, end int, e *list.Element, tok token.Token) {
	b := e.Value.(*ele).b

	if start == end {
		panic(fmt.Errorf("undefine symbol {%v,%v}", start, end))
	}

	if end > len(b) {
		panic(fmt.Errorf("outside of slice {%v,%v}", end, len(b)))
	}

	if start == 0 && end == len(b) {
		e.Value.(*ele).tok = tok
		return
	}

	if start == 0 { // comment at the first line
		present, aft := b[:end], b[end:]

		e.Value.(*ele).b = present
		e.Value.(*ele).tok = tok

		if len(aft) > 0 {
			s.eles.InsertAfter(&ele{
				tok: undefine,
				b:   aft,
				pos: position{
					line: e.Value.(*ele).pos.line,
					col:  e.Value.(*ele).pos.col + start,
				},
			}, e)
		}
		return
	}

	if end == len(b) {
		// start is not 0
		// end is end of slice
		bef, present := b[:start], b[start:]
		s.eles.InsertAfter(&ele{
			tok: tok,
			b:   present,
			pos: position{
				line: e.Value.(*ele).pos.line,
				col:  e.Value.(*ele).pos.col + start,
			},
		}, e)
		e.Value.(*ele).tok = undefine
		e.Value.(*ele).b = bef
		return
	}

	// start is not 0
	// end is not end

	bef, present, aft := b[:start], b[start:end], b[end:]

	e.Value.(*ele).tok = undefine
	e.Value.(*ele).b = bef

	s.eles.InsertAfter(&ele{
		tok: undefine,
		b:   aft,
		pos: position{
			line: e.Value.(*ele).pos.line,
			col:  e.Value.(*ele).pos.col + end,
		},
	}, e)

	s.eles.InsertAfter(&ele{
		tok: tok,
		b:   present,
		pos: position{
			line: e.Value.(*ele).pos.line,
			col:  e.Value.(*ele).pos.col + start,
		},
	}, e)
}

// separate strings
func (s *elScan) scanStrings() {
	for e := s.eles.Front(); e != nil; e = e.Next() {
		switch e.Value.(*ele).tok {
		case undefine:
			for j, ch := range e.Value.(*ele).b {
				if ch == '"' {
					b := e.Value.(*ele).b
					var end int
					for end = j + 1; end < len(b) && b[end] != '"'; end++ {
					}
					s.extract(j, end+1, e, token.STRING)
					break
				} else if ch == '\'' {
					b := e.Value.(*ele).b
					var end int
					for end = j + 1; end < len(b) && b[end] != '\''; end++ {
					}
					s.extract(j, end+1, e, token.STRING)
					break
				}
			}
		}
	}
}

// scanTokenWithPoint for identification
func (s *elScan) scanTokenWithPoint() {
	// Example of possible error:
	// IF ( 2.LE.1) ...
	//       |
	//       +- error here, because it is not value "2."
	//          it is value "2"

	entities := []struct {
		tok     token.Token
		pattern string
	}{
		// operation with points
		{tok: token.LSS, pattern: ".LT."},
		{tok: token.GTR, pattern: ".GT."},
		{tok: token.LEQ, pattern: ".LE."},
		{tok: token.GEQ, pattern: ".GE."},
		{tok: token.NOT, pattern: ".NOT."},
		{tok: token.NEQ, pattern: ".NE."},
		{tok: token.EQL, pattern: ".EQ."},
		{tok: token.LAND, pattern: ".AND."},
		{tok: token.LOR, pattern: ".OR."},
		{tok: token.IDENT, pattern: ".TRUE."},
		{tok: token.IDENT, pattern: ".true."},
		{tok: token.IDENT, pattern: ".FALSE."},
		{tok: token.IDENT, pattern: ".false."},

		// !=
		{tok: token.NEQ, pattern: "/="},
		// other
		{tok: DOUBLE_COLON, pattern: "::"},
		{tok: token.COLON, pattern: ":"},
		{tok: token.COMMA, pattern: ","},
		{tok: token.LPAREN, pattern: "("},
		{tok: token.RPAREN, pattern: ")"},
		{tok: token.ASSIGN, pattern: "="},
		{tok: token.GTR, pattern: ">"},
		{tok: token.LSS, pattern: "<"},
		{tok: DOLLAR, pattern: "$"},
		// stars
		{tok: DOUBLE_STAR, pattern: "**"},
		{tok: token.MUL, pattern: "*"},
		// devs
		{tok: STRING_CONCAT, pattern: "//"},
		{tok: token.QUO, pattern: "/"},
	}

A:
	var changed bool
	for e := s.eles.Front(); e != nil; e = e.Next() {
		if e.Value.(*ele).tok != undefine {
			continue
		}
		for _, ent := range entities {
			ind := bytes.Index(e.Value.(*ele).b, []byte(ent.pattern))
			if ind < 0 {
				continue
			}
			s.extract(ind, ind+len(ent.pattern), e, ent.tok)
			changed = true
			break
		}
	}
	if changed {
		goto A
	}
}

// postprocessor
func (s *elScan) postprocessor() {

	// From:
	//  END SUBROUTINE
	//  END IF
	// To:
	//  END
	for e := s.eles.Front(); e != nil; e = e.Next() {
		if e.Value.(*ele).tok == END {
			for n := e.Next(); n != nil; n = e.Next() {
				if n.Value.(*ele).tok != NEW_LINE {
					s.eles.Remove(n)
				} else {
					break
				}
			}
		}
	}

	// From:
	//   ELSEIF
	// To:
	//   ELSE IF
	for e := s.eles.Front(); e != nil; e = e.Next() {
		if e.Value.(*ele).tok == ELSEIF {
			e.Value.(*ele).tok, e.Value.(*ele).b = token.ELSE, []byte("ELSE")
			s.eles.InsertAfter(&ele{
				tok: token.IF,
				b:   []byte("IF"),
			}, e)
		}
	}

	// From:
	//   /= token.NEQ
	// To:
	//   != token.NEQ
	for e := s.eles.Front(); e != nil; e = e.Next() {
		if e.Value.(*ele).tok == token.NEQ {
			e.Value.(*ele).tok, e.Value.(*ele).b = token.NEQ, []byte("!=")
		}
	}

	// replace string concatenation
	for e := s.eles.Front(); e != nil; e = e.Next() {
		if e.Value.(*ele).tok == STRING_CONCAT {
			e.Value.(*ele).tok, e.Value.(*ele).b = token.ADD, []byte("+")
		}
	}

	// Multiline expression
	// if any in column 6, then merge lines
multi:
	for e := s.eles.Front(); e != nil; e = e.Next() {
		if e.Value.(*ele).tok == NEW_LINE {
			n := e.Next()
			if n == nil {
				continue
			}
			if n.Value.(*ele).pos.col == 6 {
				s.eles.Remove(e)
				s.eles.Remove(n)
				goto multi
			}
		}
	}

	// Multiline function arguments
	// From:
	//  9999 FORMAT ( ' ** On entry to ' , A , ' parameter number ' , I2 , ' had ' ,
	//  'an illegal value' )
	// To:
	//  9999 FORMAT ( ' ** On entry to ' , A , ' parameter number ' , I2 , ' had ' , 'an illegal value' )
	for e := s.eles.Front(); e != nil; e = e.Next() {
		if e.Value.(*ele).tok == token.COMMA {
			n := e.Next()
			if n == nil {
				continue
			}
			if n.Value.(*ele).tok == NEW_LINE {
				s.eles.Remove(n)
			}
		}
	}

	// Simplification of PARAMETER:
	// From:
	//  PARAMETER ( ONE = ( 1.0E+0 , 0.0E+0 )  , ZERO = 0.0E+0 )
	// To:
	//  ONE = ( 1.0E+0 , 0.0E+0 )
	//  ZERO = 0.0E+0
	//
	for e := s.eles.Front(); e != nil; e = e.Next() {
		if e.Value.(*ele).tok != NEW_LINE {
			continue
		}
		e = e.Next()
		if e == nil {
			break
		}
		if e.Value.(*ele).tok != PARAMETER {
			continue
		}
		// replace PARAMETER to NEW_LINE
		n := e.Next()
		e.Value.(*ele).b, e.Value.(*ele).tok = []byte{'\n'}, NEW_LINE
		e = n
		// replace ( to NEW_LINE
		if e.Value.(*ele).tok != token.LPAREN {
			panic("is not LPAREN")
		}
		e.Value.(*ele).b, e.Value.(*ele).tok = []byte{'\n'}, NEW_LINE
		e = e.Next()
		// find end )
		counter := 1
		for ; e != nil; e = e.Next() {
			if e.Value.(*ele).tok == NEW_LINE {
				// panic(fmt.Errorf("NEW_LINE is not accepted"))
				break
			}
			if e.Value.(*ele).tok == token.LPAREN {
				counter++
			}
			if e.Value.(*ele).tok == token.RPAREN {
				counter--
			}
			if counter == 1 && e.Value.(*ele).tok == token.COMMA {
				// replace , to NEW_LINE
				e.Value.(*ele).b, e.Value.(*ele).tok = []byte{'\n'}, NEW_LINE
			}
			if counter == 0 {
				if e.Value.(*ele).tok != token.RPAREN {
					panic("Must RPAREN")
				}
				// replace ) to NEW_LINE
				e.Value.(*ele).b, e.Value.(*ele).tok = []byte{'\n'}, NEW_LINE
				break
			}
		}
	}

	// .TRUE. to true
	// .FALSE. to false
	for e := s.eles.Front(); e != nil; e = e.Next() {
		if e.Value.(*ele).tok != token.IDENT {
			continue
		}
		switch strings.ToUpper(string(e.Value.(*ele).b)) {
		case ".TRUE.":
			e.Value.(*ele).b = []byte("true")
		case ".FALSE.":
			e.Value.(*ele).b = []byte("false")
		}
	}
}

func (s *elScan) scanTokens() {
	entities := []struct {
		tok     token.Token
		pattern []string
	}{
		{tok: SUBROUTINE, pattern: []string{"SUBROUTINE"}},
		{tok: IMPLICIT, pattern: []string{"IMPLICIT"}},
		{tok: INTEGER, pattern: []string{"INTEGER"}},
		{tok: CHARACTER, pattern: []string{"CHARACTER"}},
		{tok: LOGICAL, pattern: []string{"LOGICAL"}},
		{tok: COMPLEX, pattern: []string{"COMPLEX"}},
		{tok: REAL, pattern: []string{"REAL"}},
		{tok: DATA, pattern: []string{"DATA"}},
		{tok: EXTERNAL, pattern: []string{"EXTERNAL"}},
		{tok: END, pattern: []string{"END", "ENDDO"}},
		{tok: DO, pattern: []string{"DO"}},
		{tok: DOUBLE, pattern: []string{"DOUBLE"}},
		{tok: FUNCTION, pattern: []string{"FUNCTION"}},
		{tok: token.IF, pattern: []string{"IF"}},
		{tok: token.ELSE, pattern: []string{"ELSE"}},
		{tok: token.CONTINUE, pattern: []string{"CONTINUE"}},
		{tok: CALL, pattern: []string{"CALL"}},
		{tok: THEN, pattern: []string{"THEN"}},
		{tok: token.RETURN, pattern: []string{"RETURN"}},
		{tok: WRITE, pattern: []string{"WRITE"}},
		{tok: WHILE, pattern: []string{"WHILE"}},
		{tok: PARAMETER, pattern: []string{"PARAMETER"}},
		{tok: PROGRAM, pattern: []string{"PROGRAM"}},
		{tok: PRECISION, pattern: []string{"PRECISION"}},
		{tok: INTRINSIC, pattern: []string{"INTRINSIC"}},
		{tok: FORMAT, pattern: []string{"FORMAT"}},
		{tok: STOP, pattern: []string{"STOP"}},
		{tok: token.GOTO, pattern: []string{"GOTO"}},
		{tok: ELSEIF, pattern: []string{"ELSEIF"}},
	}
A:
	var changed bool
	for e := s.eles.Front(); e != nil; e = e.Next() {
		for _, ent := range entities {
			for _, pat := range ent.pattern {
				switch e.Value.(*ele).tok {
				case undefine:
					index := bytes.Index(
						bytes.ToUpper([]byte(string(e.Value.(*ele).b))),
						bytes.ToUpper([]byte(pat)))
					if index < 0 {
						continue
					}

					var found bool
					if index == 0 {
						if len(e.Value.(*ele).b) == len(pat) ||
							!(isLetter(rune(e.Value.(*ele).b[len(pat)])) ||
								isDigit(rune(e.Value.(*ele).b[len(pat)]))) {
							found = true
						}
					}
					if index > 0 {
						if e.Value.(*ele).b[index-1] == ' ' &&
							(len(e.Value.(*ele).b) == index+len(pat) ||
								!isLetter(rune(e.Value.(*ele).b[index+len(pat)]))) {
							found = true
						}
					}

					if found {
						s.extract(index, index+len(pat), e, ent.tok)
						changed = true
						goto en
					}
				}
			}
		}
	en:
	}
	if changed {
		goto A
	}
}

func (s *elScan) scanTokensAfter() {
	entities := []struct {
		tok     token.Token
		pattern []string
	}{
		{tok: token.PERIOD, pattern: []string{"."}},
		{tok: token.ADD, pattern: []string{"+"}},
		{tok: token.SUB, pattern: []string{"-"}},
	}
A:
	var changed bool
	for e := s.eles.Front(); e != nil; e = e.Next() {
		for _, ent := range entities {
			for _, pat := range ent.pattern {
				switch e.Value.(*ele).tok {
				case undefine:
					index := bytes.Index([]byte(string(e.Value.(*ele).b)), []byte(pat))
					if index < 0 {
						continue
					}
					s.extract(index, index+len(pat), e, ent.tok)
					changed = true
					goto en
				}
			}
		}
	en:
	}
	if changed {
		goto A
	}
}

// remove empty undefine tokens
func (s *elScan) scanEmpty() {
empty:
	var again bool
	for e := s.eles.Front(); e != nil; e = e.Next() {
		switch e.Value.(*ele).tok {
		case undefine:
			if len(e.Value.(*ele).b) == 0 {
				n := e.Next()
				s.eles.Remove(e)
				e = n
				again = true
				continue
			}
			if len(bytes.TrimSpace([]byte(string(e.Value.(*ele).b)))) == 0 {
				n := e.Next()
				s.eles.Remove(e)
				e = n
				again = true
				continue
			}
			es := e.Value.(*ele).Split()
			if len(es) == 1 && bytes.Equal(e.Value.(*ele).b, es[0].b) {
				continue
			}
			for i := len(es) - 1; i >= 1; i-- {
				s.eles.InsertAfter(&es[i], e)
			}
			if len(es) == 0 {
				n := e.Next()
				s.eles.Remove(e)
				e = n
				again = true
				continue
			}
			e.Value.(*ele).b = es[0].b
			e.Value.(*ele).pos = es[0].pos
			again = true
			continue
		}
	}
	if again {
		goto empty
	}
}

func (s *elScan) scanNumbers() {
numb:
	for e := s.eles.Front(); e != nil; e = e.Next() {
		switch e.Value.(*ele).tok {
		case undefine:
			// Examples:
			// +0.000E4
			// -44
			// 2
			// +123.213545Q-5
			// 12.324e34
			// 4E23
			// STAGES:        //
			//  1. Digits     // must
			//  2. Point      // must
			//  3. Digits     // maybe
			//  4. Exponenta  // maybe
			//  5. Sign       // maybe
			//  6. Digits     // maybe
			for st := 0; st < len(e.Value.(*ele).b); st++ {
				if isDigit(rune(e.Value.(*ele).b[st])) {
					var en int
					for en = st; en < len(e.Value.(*ele).b); en++ {
						if !isDigit(rune(e.Value.(*ele).b[en])) {
							break
						}
					}
					if en < len(e.Value.(*ele).b) && (e.Value.(*ele).b[en] == '.' ||
						e.Value.(*ele).b[en] == 'E' || e.Value.(*ele).b[en] == 'e' ||
						e.Value.(*ele).b[en] == 'D' || e.Value.(*ele).b[en] == 'd' ||
						e.Value.(*ele).b[en] == 'Q' || e.Value.(*ele).b[en] == 'q') {
						// FLOAT
						if e.Value.(*ele).b[en] == '.' {
							for en = en + 1; en < len(e.Value.(*ele).b); en++ {
								if !isDigit(rune(e.Value.(*ele).b[en])) {
									break
								}
							}
						}
						if en < len(e.Value.(*ele).b) &&
							(e.Value.(*ele).b[en] == 'E' || e.Value.(*ele).b[en] == 'e' ||
								e.Value.(*ele).b[en] == 'D' || e.Value.(*ele).b[en] == 'd' ||
								e.Value.(*ele).b[en] == 'Q' || e.Value.(*ele).b[en] == 'q') {
							if en+1 < len(e.Value.(*ele).b) &&
								(e.Value.(*ele).b[en+1] == '+' || e.Value.(*ele).b[en+1] == '-') {
								en++
							}
							for en = en + 1; en < len(e.Value.(*ele).b); en++ {
								if !isDigit(rune(e.Value.(*ele).b[en])) {
									break
								}
							}
						}
						s.extract(st, en, e, token.FLOAT)
						goto numb
					} else {
						// INT
						s.extract(st, en, e, token.INT)
						goto numb
					}
				} else {
					for ; st < len(e.Value.(*ele).b); st++ {
						if e.Value.(*ele).b[st] != '_' &&
							!isDigit(rune(e.Value.(*ele).b[st])) &&
							!isLetter(rune(e.Value.(*ele).b[st])) {
							break
						}
					}
					if st >= len(e.Value.(*ele).b) {
						break
					}
				}
			}
		}
	}
}

func (s *elScan) scanGoto() {
G:
	for e := s.eles.Front(); e != nil; e = e.Next() {
		if !(e.Value.(*ele).tok == token.IDENT && string(e.Value.(*ele).b) == "GO") {
			continue
		}
		n := e.Next()
		if n == nil {
			continue
		}
		if !(n.Value.(*ele).tok == token.IDENT && string(n.Value.(*ele).b) == "TO") {
			continue
		}
		e.Value.(*ele).tok = token.GOTO
		e.Value.(*ele).b = []byte("goto")
		s.eles.Remove(n)
		goto G
	}
}
