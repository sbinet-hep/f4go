package ast

import "strings"

type String_cst struct {
	Type string
	Strg string
	Lngt string
}

func (a String_cst) GenNodeName() string {
	return "String_cst "
}

func parse_string_cst(line string) (n Node) {
	groups := groupsFromRegex(
		`
	type:(?P<type>.*) +
	strg:(?P<strg>.*) +
	lngt:(?P<lngt>.*) *
	`,
		line,
	)
	return String_cst{
		Type: strings.TrimSpace(groups["type"]),
		Strg: strings.TrimSpace(groups["strg"]),
		Lngt: strings.TrimSpace(groups["lngt"]),
	}
}
