// Code generated from tools/es-language-parser/upstream/EqlBase.g4 by ANTLR 4.13.1. DO NOT EDIT.

/*
 * Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
 * or more contributor license agreements. Licensed under the Elastic License
 * 2.0; you may not use this file except in compliance with the Elastic License
 * 2.0.
 */

package generated // EqlBase
import (
	"fmt"
	"strconv"
	"sync"

	"github.com/antlr4-go/antlr/v4"
)

// Suppress unused import errors
var _ = fmt.Printf
var _ = strconv.Itoa
var _ = sync.Once{}

type EqlBaseParser struct {
	*antlr.BaseParser
}

var EqlBaseParserStaticData struct {
	once                   sync.Once
	serializedATN          []int32
	LiteralNames           []string
	SymbolicNames          []string
	RuleNames              []string
	PredictionContextCache *antlr.PredictionContextCache
	atn                    *antlr.ATN
	decisionToDFA          []*antlr.DFA
}

func eqlbaseParserInit() {
	staticData := &EqlBaseParserStaticData
	staticData.LiteralNames = []string{
		"", "'and'", "'any'", "'by'", "'false'", "'in'", "'in~'", "'join'",
		"'like'", "'like~'", "'maxspan'", "'not'", "'null'", "'of'", "'or'",
		"'regex'", "'regex~'", "'sample'", "'sequence'", "'true'", "'until'",
		"'where'", "'with'", "':'", "'='", "'=='", "'!='", "'<'", "'<='", "'>'",
		"'>='", "'+'", "'-'", "'*'", "'/'", "'%'", "'.'", "','", "'['", "']'",
		"'('", "')'", "'|'", "'?'", "'!['",
	}
	staticData.SymbolicNames = []string{
		"", "AND", "ANY", "BY", "FALSE", "IN", "IN_INSENSITIVE", "JOIN", "LIKE",
		"LIKE_INSENSITIVE", "MAXSPAN", "NOT", "NULL", "OF", "OR", "REGEX", "REGEX_INSENSITIVE",
		"SAMPLE", "SEQUENCE", "TRUE", "UNTIL", "WHERE", "WITH", "SEQ", "ASGN",
		"EQ", "NEQ", "LT", "LTE", "GT", "GTE", "PLUS", "MINUS", "ASTERISK",
		"SLASH", "PERCENT", "DOT", "COMMA", "LB", "RB", "LP", "RP", "PIPE",
		"OPTIONAL", "MISSING_EVENT_OPEN", "STRING", "INTEGER_VALUE", "DECIMAL_VALUE",
		"IDENTIFIER", "QUOTED_IDENTIFIER", "TILDE_IDENTIFIER", "LINE_COMMENT",
		"BRACKETED_COMMENT", "WS",
	}
	staticData.RuleNames = []string{
		"singleStatement", "singleExpression", "statement", "query", "sequenceParams",
		"sequence", "sample", "join", "pipe", "joinKeys", "joinTerm", "sequenceTerm",
		"subquery", "eventQuery", "eventFilter", "expression", "booleanExpression",
		"valueExpression", "operatorExpression", "predicate", "primaryExpression",
		"functionExpression", "functionName", "constant", "comparisonOperator",
		"booleanValue", "qualifiedName", "identifier", "timeUnit", "number",
		"string", "eventValue",
	}
	staticData.PredictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 1, 53, 327, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2, 4, 7,
		4, 2, 5, 7, 5, 2, 6, 7, 6, 2, 7, 7, 7, 2, 8, 7, 8, 2, 9, 7, 9, 2, 10, 7,
		10, 2, 11, 7, 11, 2, 12, 7, 12, 2, 13, 7, 13, 2, 14, 7, 14, 2, 15, 7, 15,
		2, 16, 7, 16, 2, 17, 7, 17, 2, 18, 7, 18, 2, 19, 7, 19, 2, 20, 7, 20, 2,
		21, 7, 21, 2, 22, 7, 22, 2, 23, 7, 23, 2, 24, 7, 24, 2, 25, 7, 25, 2, 26,
		7, 26, 2, 27, 7, 27, 2, 28, 7, 28, 2, 29, 7, 29, 2, 30, 7, 30, 2, 31, 7,
		31, 1, 0, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 2, 1, 2, 5, 2, 73, 8, 2, 10,
		2, 12, 2, 76, 9, 2, 1, 3, 1, 3, 1, 3, 1, 3, 3, 3, 82, 8, 3, 1, 4, 1, 4,
		1, 4, 1, 4, 1, 4, 1, 5, 1, 5, 1, 5, 3, 5, 92, 8, 5, 1, 5, 1, 5, 3, 5, 96,
		8, 5, 3, 5, 98, 8, 5, 1, 5, 4, 5, 101, 8, 5, 11, 5, 12, 5, 102, 1, 5, 1,
		5, 3, 5, 107, 8, 5, 1, 6, 1, 6, 3, 6, 111, 8, 6, 1, 6, 4, 6, 114, 8, 6,
		11, 6, 12, 6, 115, 1, 7, 1, 7, 3, 7, 120, 8, 7, 1, 7, 1, 7, 4, 7, 124,
		8, 7, 11, 7, 12, 7, 125, 1, 7, 1, 7, 3, 7, 130, 8, 7, 1, 8, 1, 8, 1, 8,
		1, 8, 1, 8, 5, 8, 137, 8, 8, 10, 8, 12, 8, 140, 9, 8, 3, 8, 142, 8, 8,
		1, 9, 1, 9, 1, 9, 1, 9, 5, 9, 148, 8, 9, 10, 9, 12, 9, 151, 9, 9, 1, 10,
		1, 10, 3, 10, 155, 8, 10, 1, 11, 1, 11, 3, 11, 159, 8, 11, 1, 11, 1, 11,
		1, 11, 1, 11, 3, 11, 165, 8, 11, 1, 12, 1, 12, 1, 12, 1, 12, 1, 13, 1,
		13, 1, 14, 1, 14, 3, 14, 175, 8, 14, 1, 14, 1, 14, 1, 14, 1, 15, 1, 15,
		1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 3, 16, 189, 8, 16, 1,
		16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 5, 16, 197, 8, 16, 10, 16, 12, 16,
		200, 9, 16, 1, 17, 1, 17, 1, 17, 1, 17, 1, 17, 3, 17, 207, 8, 17, 1, 18,
		1, 18, 1, 18, 3, 18, 212, 8, 18, 1, 18, 1, 18, 3, 18, 216, 8, 18, 1, 18,
		1, 18, 1, 18, 1, 18, 1, 18, 1, 18, 5, 18, 224, 8, 18, 10, 18, 12, 18, 227,
		9, 18, 1, 19, 3, 19, 230, 8, 19, 1, 19, 1, 19, 1, 19, 1, 19, 1, 19, 5,
		19, 237, 8, 19, 10, 19, 12, 19, 240, 9, 19, 1, 19, 1, 19, 1, 19, 1, 19,
		1, 19, 1, 19, 1, 19, 1, 19, 1, 19, 5, 19, 251, 8, 19, 10, 19, 12, 19, 254,
		9, 19, 1, 19, 1, 19, 3, 19, 258, 8, 19, 1, 20, 1, 20, 1, 20, 1, 20, 1,
		20, 1, 20, 1, 20, 3, 20, 267, 8, 20, 1, 21, 1, 21, 1, 21, 1, 21, 1, 21,
		5, 21, 274, 8, 21, 10, 21, 12, 21, 277, 9, 21, 3, 21, 279, 8, 21, 1, 21,
		1, 21, 1, 22, 1, 22, 1, 23, 1, 23, 1, 23, 1, 23, 3, 23, 289, 8, 23, 1,
		24, 1, 24, 1, 25, 1, 25, 1, 26, 3, 26, 296, 8, 26, 1, 26, 1, 26, 1, 26,
		1, 26, 1, 26, 4, 26, 303, 8, 26, 11, 26, 12, 26, 304, 1, 26, 5, 26, 308,
		8, 26, 10, 26, 12, 26, 311, 9, 26, 1, 27, 1, 27, 1, 28, 1, 28, 3, 28, 317,
		8, 28, 1, 29, 1, 29, 3, 29, 321, 8, 29, 1, 30, 1, 30, 1, 31, 1, 31, 1,
		31, 0, 2, 32, 36, 32, 0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26,
		28, 30, 32, 34, 36, 38, 40, 42, 44, 46, 48, 50, 52, 54, 56, 58, 60, 62,
		0, 10, 2, 0, 38, 38, 44, 44, 1, 0, 31, 32, 1, 0, 33, 35, 1, 0, 5, 6, 3,
		0, 8, 9, 15, 16, 23, 23, 2, 0, 48, 48, 50, 50, 1, 0, 25, 30, 2, 0, 4, 4,
		19, 19, 1, 0, 48, 49, 2, 0, 45, 45, 48, 48, 344, 0, 64, 1, 0, 0, 0, 2,
		67, 1, 0, 0, 0, 4, 70, 1, 0, 0, 0, 6, 81, 1, 0, 0, 0, 8, 83, 1, 0, 0, 0,
		10, 88, 1, 0, 0, 0, 12, 108, 1, 0, 0, 0, 14, 117, 1, 0, 0, 0, 16, 131,
		1, 0, 0, 0, 18, 143, 1, 0, 0, 0, 20, 152, 1, 0, 0, 0, 22, 156, 1, 0, 0,
		0, 24, 166, 1, 0, 0, 0, 26, 170, 1, 0, 0, 0, 28, 174, 1, 0, 0, 0, 30, 179,
		1, 0, 0, 0, 32, 188, 1, 0, 0, 0, 34, 206, 1, 0, 0, 0, 36, 215, 1, 0, 0,
		0, 38, 257, 1, 0, 0, 0, 40, 266, 1, 0, 0, 0, 42, 268, 1, 0, 0, 0, 44, 282,
		1, 0, 0, 0, 46, 288, 1, 0, 0, 0, 48, 290, 1, 0, 0, 0, 50, 292, 1, 0, 0,
		0, 52, 295, 1, 0, 0, 0, 54, 312, 1, 0, 0, 0, 56, 314, 1, 0, 0, 0, 58, 320,
		1, 0, 0, 0, 60, 322, 1, 0, 0, 0, 62, 324, 1, 0, 0, 0, 64, 65, 3, 4, 2,
		0, 65, 66, 5, 0, 0, 1, 66, 1, 1, 0, 0, 0, 67, 68, 3, 30, 15, 0, 68, 69,
		5, 0, 0, 1, 69, 3, 1, 0, 0, 0, 70, 74, 3, 6, 3, 0, 71, 73, 3, 16, 8, 0,
		72, 71, 1, 0, 0, 0, 73, 76, 1, 0, 0, 0, 74, 72, 1, 0, 0, 0, 74, 75, 1,
		0, 0, 0, 75, 5, 1, 0, 0, 0, 76, 74, 1, 0, 0, 0, 77, 82, 3, 10, 5, 0, 78,
		82, 3, 14, 7, 0, 79, 82, 3, 26, 13, 0, 80, 82, 3, 12, 6, 0, 81, 77, 1,
		0, 0, 0, 81, 78, 1, 0, 0, 0, 81, 79, 1, 0, 0, 0, 81, 80, 1, 0, 0, 0, 82,
		7, 1, 0, 0, 0, 83, 84, 5, 22, 0, 0, 84, 85, 5, 10, 0, 0, 85, 86, 5, 24,
		0, 0, 86, 87, 3, 56, 28, 0, 87, 9, 1, 0, 0, 0, 88, 97, 5, 18, 0, 0, 89,
		91, 3, 18, 9, 0, 90, 92, 3, 8, 4, 0, 91, 90, 1, 0, 0, 0, 91, 92, 1, 0,
		0, 0, 92, 98, 1, 0, 0, 0, 93, 95, 3, 8, 4, 0, 94, 96, 3, 18, 9, 0, 95,
		94, 1, 0, 0, 0, 95, 96, 1, 0, 0, 0, 96, 98, 1, 0, 0, 0, 97, 89, 1, 0, 0,
		0, 97, 93, 1, 0, 0, 0, 97, 98, 1, 0, 0, 0, 98, 100, 1, 0, 0, 0, 99, 101,
		3, 22, 11, 0, 100, 99, 1, 0, 0, 0, 101, 102, 1, 0, 0, 0, 102, 100, 1, 0,
		0, 0, 102, 103, 1, 0, 0, 0, 103, 106, 1, 0, 0, 0, 104, 105, 5, 20, 0, 0,
		105, 107, 3, 22, 11, 0, 106, 104, 1, 0, 0, 0, 106, 107, 1, 0, 0, 0, 107,
		11, 1, 0, 0, 0, 108, 110, 5, 17, 0, 0, 109, 111, 3, 18, 9, 0, 110, 109,
		1, 0, 0, 0, 110, 111, 1, 0, 0, 0, 111, 113, 1, 0, 0, 0, 112, 114, 3, 20,
		10, 0, 113, 112, 1, 0, 0, 0, 114, 115, 1, 0, 0, 0, 115, 113, 1, 0, 0, 0,
		115, 116, 1, 0, 0, 0, 116, 13, 1, 0, 0, 0, 117, 119, 5, 7, 0, 0, 118, 120,
		3, 18, 9, 0, 119, 118, 1, 0, 0, 0, 119, 120, 1, 0, 0, 0, 120, 121, 1, 0,
		0, 0, 121, 123, 3, 20, 10, 0, 122, 124, 3, 20, 10, 0, 123, 122, 1, 0, 0,
		0, 124, 125, 1, 0, 0, 0, 125, 123, 1, 0, 0, 0, 125, 126, 1, 0, 0, 0, 126,
		129, 1, 0, 0, 0, 127, 128, 5, 20, 0, 0, 128, 130, 3, 20, 10, 0, 129, 127,
		1, 0, 0, 0, 129, 130, 1, 0, 0, 0, 130, 15, 1, 0, 0, 0, 131, 132, 5, 42,
		0, 0, 132, 141, 5, 48, 0, 0, 133, 138, 3, 32, 16, 0, 134, 135, 5, 37, 0,
		0, 135, 137, 3, 32, 16, 0, 136, 134, 1, 0, 0, 0, 137, 140, 1, 0, 0, 0,
		138, 136, 1, 0, 0, 0, 138, 139, 1, 0, 0, 0, 139, 142, 1, 0, 0, 0, 140,
		138, 1, 0, 0, 0, 141, 133, 1, 0, 0, 0, 141, 142, 1, 0, 0, 0, 142, 17, 1,
		0, 0, 0, 143, 144, 5, 3, 0, 0, 144, 149, 3, 30, 15, 0, 145, 146, 5, 37,
		0, 0, 146, 148, 3, 30, 15, 0, 147, 145, 1, 0, 0, 0, 148, 151, 1, 0, 0,
		0, 149, 147, 1, 0, 0, 0, 149, 150, 1, 0, 0, 0, 150, 19, 1, 0, 0, 0, 151,
		149, 1, 0, 0, 0, 152, 154, 3, 24, 12, 0, 153, 155, 3, 18, 9, 0, 154, 153,
		1, 0, 0, 0, 154, 155, 1, 0, 0, 0, 155, 21, 1, 0, 0, 0, 156, 158, 3, 24,
		12, 0, 157, 159, 3, 18, 9, 0, 158, 157, 1, 0, 0, 0, 158, 159, 1, 0, 0,
		0, 159, 164, 1, 0, 0, 0, 160, 161, 5, 22, 0, 0, 161, 162, 5, 48, 0, 0,
		162, 163, 5, 24, 0, 0, 163, 165, 3, 58, 29, 0, 164, 160, 1, 0, 0, 0, 164,
		165, 1, 0, 0, 0, 165, 23, 1, 0, 0, 0, 166, 167, 7, 0, 0, 0, 167, 168, 3,
		28, 14, 0, 168, 169, 5, 39, 0, 0, 169, 25, 1, 0, 0, 0, 170, 171, 3, 28,
		14, 0, 171, 27, 1, 0, 0, 0, 172, 175, 5, 2, 0, 0, 173, 175, 3, 62, 31,
		0, 174, 172, 1, 0, 0, 0, 174, 173, 1, 0, 0, 0, 175, 176, 1, 0, 0, 0, 176,
		177, 5, 21, 0, 0, 177, 178, 3, 30, 15, 0, 178, 29, 1, 0, 0, 0, 179, 180,
		3, 32, 16, 0, 180, 31, 1, 0, 0, 0, 181, 182, 6, 16, -1, 0, 182, 183, 5,
		11, 0, 0, 183, 189, 3, 32, 16, 5, 184, 185, 5, 48, 0, 0, 185, 186, 5, 13,
		0, 0, 186, 189, 3, 24, 12, 0, 187, 189, 3, 34, 17, 0, 188, 181, 1, 0, 0,
		0, 188, 184, 1, 0, 0, 0, 188, 187, 1, 0, 0, 0, 189, 198, 1, 0, 0, 0, 190,
		191, 10, 2, 0, 0, 191, 192, 5, 1, 0, 0, 192, 197, 3, 32, 16, 3, 193, 194,
		10, 1, 0, 0, 194, 195, 5, 14, 0, 0, 195, 197, 3, 32, 16, 2, 196, 190, 1,
		0, 0, 0, 196, 193, 1, 0, 0, 0, 197, 200, 1, 0, 0, 0, 198, 196, 1, 0, 0,
		0, 198, 199, 1, 0, 0, 0, 199, 33, 1, 0, 0, 0, 200, 198, 1, 0, 0, 0, 201,
		207, 3, 36, 18, 0, 202, 203, 3, 36, 18, 0, 203, 204, 3, 48, 24, 0, 204,
		205, 3, 36, 18, 0, 205, 207, 1, 0, 0, 0, 206, 201, 1, 0, 0, 0, 206, 202,
		1, 0, 0, 0, 207, 35, 1, 0, 0, 0, 208, 209, 6, 18, -1, 0, 209, 211, 3, 40,
		20, 0, 210, 212, 3, 38, 19, 0, 211, 210, 1, 0, 0, 0, 211, 212, 1, 0, 0,
		0, 212, 216, 1, 0, 0, 0, 213, 214, 7, 1, 0, 0, 214, 216, 3, 36, 18, 3,
		215, 208, 1, 0, 0, 0, 215, 213, 1, 0, 0, 0, 216, 225, 1, 0, 0, 0, 217,
		218, 10, 2, 0, 0, 218, 219, 7, 2, 0, 0, 219, 224, 3, 36, 18, 3, 220, 221,
		10, 1, 0, 0, 221, 222, 7, 1, 0, 0, 222, 224, 3, 36, 18, 2, 223, 217, 1,
		0, 0, 0, 223, 220, 1, 0, 0, 0, 224, 227, 1, 0, 0, 0, 225, 223, 1, 0, 0,
		0, 225, 226, 1, 0, 0, 0, 226, 37, 1, 0, 0, 0, 227, 225, 1, 0, 0, 0, 228,
		230, 5, 11, 0, 0, 229, 228, 1, 0, 0, 0, 229, 230, 1, 0, 0, 0, 230, 231,
		1, 0, 0, 0, 231, 232, 7, 3, 0, 0, 232, 233, 5, 40, 0, 0, 233, 238, 3, 30,
		15, 0, 234, 235, 5, 37, 0, 0, 235, 237, 3, 30, 15, 0, 236, 234, 1, 0, 0,
		0, 237, 240, 1, 0, 0, 0, 238, 236, 1, 0, 0, 0, 238, 239, 1, 0, 0, 0, 239,
		241, 1, 0, 0, 0, 240, 238, 1, 0, 0, 0, 241, 242, 5, 41, 0, 0, 242, 258,
		1, 0, 0, 0, 243, 244, 7, 4, 0, 0, 244, 258, 3, 46, 23, 0, 245, 246, 7,
		4, 0, 0, 246, 247, 5, 40, 0, 0, 247, 252, 3, 46, 23, 0, 248, 249, 5, 37,
		0, 0, 249, 251, 3, 46, 23, 0, 250, 248, 1, 0, 0, 0, 251, 254, 1, 0, 0,
		0, 252, 250, 1, 0, 0, 0, 252, 253, 1, 0, 0, 0, 253, 255, 1, 0, 0, 0, 254,
		252, 1, 0, 0, 0, 255, 256, 5, 41, 0, 0, 256, 258, 1, 0, 0, 0, 257, 229,
		1, 0, 0, 0, 257, 243, 1, 0, 0, 0, 257, 245, 1, 0, 0, 0, 258, 39, 1, 0,
		0, 0, 259, 267, 3, 46, 23, 0, 260, 267, 3, 42, 21, 0, 261, 267, 3, 52,
		26, 0, 262, 263, 5, 40, 0, 0, 263, 264, 3, 30, 15, 0, 264, 265, 5, 41,
		0, 0, 265, 267, 1, 0, 0, 0, 266, 259, 1, 0, 0, 0, 266, 260, 1, 0, 0, 0,
		266, 261, 1, 0, 0, 0, 266, 262, 1, 0, 0, 0, 267, 41, 1, 0, 0, 0, 268, 269,
		3, 44, 22, 0, 269, 278, 5, 40, 0, 0, 270, 275, 3, 30, 15, 0, 271, 272,
		5, 37, 0, 0, 272, 274, 3, 30, 15, 0, 273, 271, 1, 0, 0, 0, 274, 277, 1,
		0, 0, 0, 275, 273, 1, 0, 0, 0, 275, 276, 1, 0, 0, 0, 276, 279, 1, 0, 0,
		0, 277, 275, 1, 0, 0, 0, 278, 270, 1, 0, 0, 0, 278, 279, 1, 0, 0, 0, 279,
		280, 1, 0, 0, 0, 280, 281, 5, 41, 0, 0, 281, 43, 1, 0, 0, 0, 282, 283,
		7, 5, 0, 0, 283, 45, 1, 0, 0, 0, 284, 289, 5, 12, 0, 0, 285, 289, 3, 58,
		29, 0, 286, 289, 3, 50, 25, 0, 287, 289, 3, 60, 30, 0, 288, 284, 1, 0,
		0, 0, 288, 285, 1, 0, 0, 0, 288, 286, 1, 0, 0, 0, 288, 287, 1, 0, 0, 0,
		289, 47, 1, 0, 0, 0, 290, 291, 7, 6, 0, 0, 291, 49, 1, 0, 0, 0, 292, 293,
		7, 7, 0, 0, 293, 51, 1, 0, 0, 0, 294, 296, 5, 43, 0, 0, 295, 294, 1, 0,
		0, 0, 295, 296, 1, 0, 0, 0, 296, 297, 1, 0, 0, 0, 297, 309, 3, 54, 27,
		0, 298, 299, 5, 36, 0, 0, 299, 308, 3, 54, 27, 0, 300, 302, 5, 38, 0, 0,
		301, 303, 5, 46, 0, 0, 302, 301, 1, 0, 0, 0, 303, 304, 1, 0, 0, 0, 304,
		302, 1, 0, 0, 0, 304, 305, 1, 0, 0, 0, 305, 306, 1, 0, 0, 0, 306, 308,
		5, 39, 0, 0, 307, 298, 1, 0, 0, 0, 307, 300, 1, 0, 0, 0, 308, 311, 1, 0,
		0, 0, 309, 307, 1, 0, 0, 0, 309, 310, 1, 0, 0, 0, 310, 53, 1, 0, 0, 0,
		311, 309, 1, 0, 0, 0, 312, 313, 7, 8, 0, 0, 313, 55, 1, 0, 0, 0, 314, 316,
		3, 58, 29, 0, 315, 317, 5, 48, 0, 0, 316, 315, 1, 0, 0, 0, 316, 317, 1,
		0, 0, 0, 317, 57, 1, 0, 0, 0, 318, 321, 5, 47, 0, 0, 319, 321, 5, 46, 0,
		0, 320, 318, 1, 0, 0, 0, 320, 319, 1, 0, 0, 0, 321, 59, 1, 0, 0, 0, 322,
		323, 5, 45, 0, 0, 323, 61, 1, 0, 0, 0, 324, 325, 7, 9, 0, 0, 325, 63, 1,
		0, 0, 0, 41, 74, 81, 91, 95, 97, 102, 106, 110, 115, 119, 125, 129, 138,
		141, 149, 154, 158, 164, 174, 188, 196, 198, 206, 211, 215, 223, 225, 229,
		238, 252, 257, 266, 275, 278, 288, 295, 304, 307, 309, 316, 320,
	}
	deserializer := antlr.NewATNDeserializer(nil)
	staticData.atn = deserializer.Deserialize(staticData.serializedATN)
	atn := staticData.atn
	staticData.decisionToDFA = make([]*antlr.DFA, len(atn.DecisionToState))
	decisionToDFA := staticData.decisionToDFA
	for index, state := range atn.DecisionToState {
		decisionToDFA[index] = antlr.NewDFA(state, index)
	}
}

// EqlBaseParserInit initializes any static state used to implement EqlBaseParser. By default the
// static state used to implement the parser is lazily initialized during the first call to
// NewEqlBaseParser(). You can call this function if you wish to initialize the static state ahead
// of time.
func EqlBaseParserInit() {
	staticData := &EqlBaseParserStaticData
	staticData.once.Do(eqlbaseParserInit)
}

// NewEqlBaseParser produces a new parser instance for the optional input antlr.TokenStream.
func NewEqlBaseParser(input antlr.TokenStream) *EqlBaseParser {
	EqlBaseParserInit()
	this := new(EqlBaseParser)
	this.BaseParser = antlr.NewBaseParser(input)
	staticData := &EqlBaseParserStaticData
	this.Interpreter = antlr.NewParserATNSimulator(this, staticData.atn, staticData.decisionToDFA, staticData.PredictionContextCache)
	this.RuleNames = staticData.RuleNames
	this.LiteralNames = staticData.LiteralNames
	this.SymbolicNames = staticData.SymbolicNames
	this.GrammarFileName = "EqlBase.g4"

	return this
}

// EqlBaseParser tokens.
const (
	EqlBaseParserEOF                = antlr.TokenEOF
	EqlBaseParserAND                = 1
	EqlBaseParserANY                = 2
	EqlBaseParserBY                 = 3
	EqlBaseParserFALSE              = 4
	EqlBaseParserIN                 = 5
	EqlBaseParserIN_INSENSITIVE     = 6
	EqlBaseParserJOIN               = 7
	EqlBaseParserLIKE               = 8
	EqlBaseParserLIKE_INSENSITIVE   = 9
	EqlBaseParserMAXSPAN            = 10
	EqlBaseParserNOT                = 11
	EqlBaseParserNULL               = 12
	EqlBaseParserOF                 = 13
	EqlBaseParserOR                 = 14
	EqlBaseParserREGEX              = 15
	EqlBaseParserREGEX_INSENSITIVE  = 16
	EqlBaseParserSAMPLE             = 17
	EqlBaseParserSEQUENCE           = 18
	EqlBaseParserTRUE               = 19
	EqlBaseParserUNTIL              = 20
	EqlBaseParserWHERE              = 21
	EqlBaseParserWITH               = 22
	EqlBaseParserSEQ                = 23
	EqlBaseParserASGN               = 24
	EqlBaseParserEQ                 = 25
	EqlBaseParserNEQ                = 26
	EqlBaseParserLT                 = 27
	EqlBaseParserLTE                = 28
	EqlBaseParserGT                 = 29
	EqlBaseParserGTE                = 30
	EqlBaseParserPLUS               = 31
	EqlBaseParserMINUS              = 32
	EqlBaseParserASTERISK           = 33
	EqlBaseParserSLASH              = 34
	EqlBaseParserPERCENT            = 35
	EqlBaseParserDOT                = 36
	EqlBaseParserCOMMA              = 37
	EqlBaseParserLB                 = 38
	EqlBaseParserRB                 = 39
	EqlBaseParserLP                 = 40
	EqlBaseParserRP                 = 41
	EqlBaseParserPIPE               = 42
	EqlBaseParserOPTIONAL           = 43
	EqlBaseParserMISSING_EVENT_OPEN = 44
	EqlBaseParserSTRING             = 45
	EqlBaseParserINTEGER_VALUE      = 46
	EqlBaseParserDECIMAL_VALUE      = 47
	EqlBaseParserIDENTIFIER         = 48
	EqlBaseParserQUOTED_IDENTIFIER  = 49
	EqlBaseParserTILDE_IDENTIFIER   = 50
	EqlBaseParserLINE_COMMENT       = 51
	EqlBaseParserBRACKETED_COMMENT  = 52
	EqlBaseParserWS                 = 53
)

// EqlBaseParser rules.
const (
	EqlBaseParserRULE_singleStatement    = 0
	EqlBaseParserRULE_singleExpression   = 1
	EqlBaseParserRULE_statement          = 2
	EqlBaseParserRULE_query              = 3
	EqlBaseParserRULE_sequenceParams     = 4
	EqlBaseParserRULE_sequence           = 5
	EqlBaseParserRULE_sample             = 6
	EqlBaseParserRULE_join               = 7
	EqlBaseParserRULE_pipe               = 8
	EqlBaseParserRULE_joinKeys           = 9
	EqlBaseParserRULE_joinTerm           = 10
	EqlBaseParserRULE_sequenceTerm       = 11
	EqlBaseParserRULE_subquery           = 12
	EqlBaseParserRULE_eventQuery         = 13
	EqlBaseParserRULE_eventFilter        = 14
	EqlBaseParserRULE_expression         = 15
	EqlBaseParserRULE_booleanExpression  = 16
	EqlBaseParserRULE_valueExpression    = 17
	EqlBaseParserRULE_operatorExpression = 18
	EqlBaseParserRULE_predicate          = 19
	EqlBaseParserRULE_primaryExpression  = 20
	EqlBaseParserRULE_functionExpression = 21
	EqlBaseParserRULE_functionName       = 22
	EqlBaseParserRULE_constant           = 23
	EqlBaseParserRULE_comparisonOperator = 24
	EqlBaseParserRULE_booleanValue       = 25
	EqlBaseParserRULE_qualifiedName      = 26
	EqlBaseParserRULE_identifier         = 27
	EqlBaseParserRULE_timeUnit           = 28
	EqlBaseParserRULE_number             = 29
	EqlBaseParserRULE_string             = 30
	EqlBaseParserRULE_eventValue         = 31
)

// ISingleStatementContext is an interface to support dynamic dispatch.
type ISingleStatementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Statement() IStatementContext
	EOF() antlr.TerminalNode

	// IsSingleStatementContext differentiates from other interfaces.
	IsSingleStatementContext()
}

type SingleStatementContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySingleStatementContext() *SingleStatementContext {
	var p = new(SingleStatementContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_singleStatement
	return p
}

func InitEmptySingleStatementContext(p *SingleStatementContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_singleStatement
}

func (*SingleStatementContext) IsSingleStatementContext() {}

func NewSingleStatementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SingleStatementContext {
	var p = new(SingleStatementContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_singleStatement

	return p
}

func (s *SingleStatementContext) GetParser() antlr.Parser { return s.parser }

func (s *SingleStatementContext) Statement() IStatementContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IStatementContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IStatementContext)
}

func (s *SingleStatementContext) EOF() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserEOF, 0)
}

func (s *SingleStatementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SingleStatementContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SingleStatementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterSingleStatement(s)
	}
}

func (s *SingleStatementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitSingleStatement(s)
	}
}

func (p *EqlBaseParser) SingleStatement() (localctx ISingleStatementContext) {
	localctx = NewSingleStatementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 0, EqlBaseParserRULE_singleStatement)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(64)
		p.Statement()
	}
	{
		p.SetState(65)
		p.Match(EqlBaseParserEOF)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// ISingleExpressionContext is an interface to support dynamic dispatch.
type ISingleExpressionContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Expression() IExpressionContext
	EOF() antlr.TerminalNode

	// IsSingleExpressionContext differentiates from other interfaces.
	IsSingleExpressionContext()
}

type SingleExpressionContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySingleExpressionContext() *SingleExpressionContext {
	var p = new(SingleExpressionContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_singleExpression
	return p
}

func InitEmptySingleExpressionContext(p *SingleExpressionContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_singleExpression
}

func (*SingleExpressionContext) IsSingleExpressionContext() {}

func NewSingleExpressionContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SingleExpressionContext {
	var p = new(SingleExpressionContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_singleExpression

	return p
}

func (s *SingleExpressionContext) GetParser() antlr.Parser { return s.parser }

func (s *SingleExpressionContext) Expression() IExpressionContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *SingleExpressionContext) EOF() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserEOF, 0)
}

func (s *SingleExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SingleExpressionContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SingleExpressionContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterSingleExpression(s)
	}
}

func (s *SingleExpressionContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitSingleExpression(s)
	}
}

func (p *EqlBaseParser) SingleExpression() (localctx ISingleExpressionContext) {
	localctx = NewSingleExpressionContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 2, EqlBaseParserRULE_singleExpression)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(67)
		p.Expression()
	}
	{
		p.SetState(68)
		p.Match(EqlBaseParserEOF)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IStatementContext is an interface to support dynamic dispatch.
type IStatementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Query() IQueryContext
	AllPipe() []IPipeContext
	Pipe(i int) IPipeContext

	// IsStatementContext differentiates from other interfaces.
	IsStatementContext()
}

type StatementContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyStatementContext() *StatementContext {
	var p = new(StatementContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_statement
	return p
}

func InitEmptyStatementContext(p *StatementContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_statement
}

func (*StatementContext) IsStatementContext() {}

func NewStatementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *StatementContext {
	var p = new(StatementContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_statement

	return p
}

func (s *StatementContext) GetParser() antlr.Parser { return s.parser }

func (s *StatementContext) Query() IQueryContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IQueryContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IQueryContext)
}

func (s *StatementContext) AllPipe() []IPipeContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IPipeContext); ok {
			len++
		}
	}

	tst := make([]IPipeContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IPipeContext); ok {
			tst[i] = t.(IPipeContext)
			i++
		}
	}

	return tst
}

func (s *StatementContext) Pipe(i int) IPipeContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IPipeContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IPipeContext)
}

func (s *StatementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *StatementContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *StatementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterStatement(s)
	}
}

func (s *StatementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitStatement(s)
	}
}

func (p *EqlBaseParser) Statement() (localctx IStatementContext) {
	localctx = NewStatementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 4, EqlBaseParserRULE_statement)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(70)
		p.Query()
	}
	p.SetState(74)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == EqlBaseParserPIPE {
		{
			p.SetState(71)
			p.Pipe()
		}

		p.SetState(76)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IQueryContext is an interface to support dynamic dispatch.
type IQueryContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Sequence() ISequenceContext
	Join() IJoinContext
	EventQuery() IEventQueryContext
	Sample() ISampleContext

	// IsQueryContext differentiates from other interfaces.
	IsQueryContext()
}

type QueryContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyQueryContext() *QueryContext {
	var p = new(QueryContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_query
	return p
}

func InitEmptyQueryContext(p *QueryContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_query
}

func (*QueryContext) IsQueryContext() {}

func NewQueryContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *QueryContext {
	var p = new(QueryContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_query

	return p
}

func (s *QueryContext) GetParser() antlr.Parser { return s.parser }

func (s *QueryContext) Sequence() ISequenceContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISequenceContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISequenceContext)
}

func (s *QueryContext) Join() IJoinContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IJoinContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IJoinContext)
}

func (s *QueryContext) EventQuery() IEventQueryContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEventQueryContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEventQueryContext)
}

func (s *QueryContext) Sample() ISampleContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISampleContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISampleContext)
}

func (s *QueryContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *QueryContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *QueryContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterQuery(s)
	}
}

func (s *QueryContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitQuery(s)
	}
}

func (p *EqlBaseParser) Query() (localctx IQueryContext) {
	localctx = NewQueryContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 6, EqlBaseParserRULE_query)
	p.SetState(81)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case EqlBaseParserSEQUENCE:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(77)
			p.Sequence()
		}

	case EqlBaseParserJOIN:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(78)
			p.Join()
		}

	case EqlBaseParserANY, EqlBaseParserSTRING, EqlBaseParserIDENTIFIER:
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(79)
			p.EventQuery()
		}

	case EqlBaseParserSAMPLE:
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(80)
			p.Sample()
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// ISequenceParamsContext is an interface to support dynamic dispatch.
type ISequenceParamsContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	WITH() antlr.TerminalNode
	MAXSPAN() antlr.TerminalNode
	ASGN() antlr.TerminalNode
	TimeUnit() ITimeUnitContext

	// IsSequenceParamsContext differentiates from other interfaces.
	IsSequenceParamsContext()
}

type SequenceParamsContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySequenceParamsContext() *SequenceParamsContext {
	var p = new(SequenceParamsContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_sequenceParams
	return p
}

func InitEmptySequenceParamsContext(p *SequenceParamsContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_sequenceParams
}

func (*SequenceParamsContext) IsSequenceParamsContext() {}

func NewSequenceParamsContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SequenceParamsContext {
	var p = new(SequenceParamsContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_sequenceParams

	return p
}

func (s *SequenceParamsContext) GetParser() antlr.Parser { return s.parser }

func (s *SequenceParamsContext) WITH() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserWITH, 0)
}

func (s *SequenceParamsContext) MAXSPAN() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserMAXSPAN, 0)
}

func (s *SequenceParamsContext) ASGN() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserASGN, 0)
}

func (s *SequenceParamsContext) TimeUnit() ITimeUnitContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITimeUnitContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ITimeUnitContext)
}

func (s *SequenceParamsContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SequenceParamsContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SequenceParamsContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterSequenceParams(s)
	}
}

func (s *SequenceParamsContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitSequenceParams(s)
	}
}

func (p *EqlBaseParser) SequenceParams() (localctx ISequenceParamsContext) {
	localctx = NewSequenceParamsContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 8, EqlBaseParserRULE_sequenceParams)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(83)
		p.Match(EqlBaseParserWITH)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

	{
		p.SetState(84)
		p.Match(EqlBaseParserMAXSPAN)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(85)
		p.Match(EqlBaseParserASGN)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(86)
		p.TimeUnit()
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// ISequenceContext is an interface to support dynamic dispatch.
type ISequenceContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetBy returns the by rule contexts.
	GetBy() IJoinKeysContext

	// GetDisallowed returns the disallowed rule contexts.
	GetDisallowed() IJoinKeysContext

	// GetUntil returns the until rule contexts.
	GetUntil() ISequenceTermContext

	// SetBy sets the by rule contexts.
	SetBy(IJoinKeysContext)

	// SetDisallowed sets the disallowed rule contexts.
	SetDisallowed(IJoinKeysContext)

	// SetUntil sets the until rule contexts.
	SetUntil(ISequenceTermContext)

	// Getter signatures
	SEQUENCE() antlr.TerminalNode
	SequenceParams() ISequenceParamsContext
	AllSequenceTerm() []ISequenceTermContext
	SequenceTerm(i int) ISequenceTermContext
	UNTIL() antlr.TerminalNode
	JoinKeys() IJoinKeysContext

	// IsSequenceContext differentiates from other interfaces.
	IsSequenceContext()
}

type SequenceContext struct {
	antlr.BaseParserRuleContext
	parser     antlr.Parser
	by         IJoinKeysContext
	disallowed IJoinKeysContext
	until      ISequenceTermContext
}

func NewEmptySequenceContext() *SequenceContext {
	var p = new(SequenceContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_sequence
	return p
}

func InitEmptySequenceContext(p *SequenceContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_sequence
}

func (*SequenceContext) IsSequenceContext() {}

func NewSequenceContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SequenceContext {
	var p = new(SequenceContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_sequence

	return p
}

func (s *SequenceContext) GetParser() antlr.Parser { return s.parser }

func (s *SequenceContext) GetBy() IJoinKeysContext { return s.by }

func (s *SequenceContext) GetDisallowed() IJoinKeysContext { return s.disallowed }

func (s *SequenceContext) GetUntil() ISequenceTermContext { return s.until }

func (s *SequenceContext) SetBy(v IJoinKeysContext) { s.by = v }

func (s *SequenceContext) SetDisallowed(v IJoinKeysContext) { s.disallowed = v }

func (s *SequenceContext) SetUntil(v ISequenceTermContext) { s.until = v }

func (s *SequenceContext) SEQUENCE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserSEQUENCE, 0)
}

func (s *SequenceContext) SequenceParams() ISequenceParamsContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISequenceParamsContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISequenceParamsContext)
}

func (s *SequenceContext) AllSequenceTerm() []ISequenceTermContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ISequenceTermContext); ok {
			len++
		}
	}

	tst := make([]ISequenceTermContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ISequenceTermContext); ok {
			tst[i] = t.(ISequenceTermContext)
			i++
		}
	}

	return tst
}

func (s *SequenceContext) SequenceTerm(i int) ISequenceTermContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISequenceTermContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISequenceTermContext)
}

func (s *SequenceContext) UNTIL() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserUNTIL, 0)
}

func (s *SequenceContext) JoinKeys() IJoinKeysContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IJoinKeysContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IJoinKeysContext)
}

func (s *SequenceContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SequenceContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SequenceContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterSequence(s)
	}
}

func (s *SequenceContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitSequence(s)
	}
}

func (p *EqlBaseParser) Sequence() (localctx ISequenceContext) {
	localctx = NewSequenceContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 10, EqlBaseParserRULE_sequence)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(88)
		p.Match(EqlBaseParserSEQUENCE)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(97)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	switch p.GetTokenStream().LA(1) {
	case EqlBaseParserBY:
		{
			p.SetState(89)

			var _x = p.JoinKeys()

			localctx.(*SequenceContext).by = _x
		}
		p.SetState(91)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == EqlBaseParserWITH {
			{
				p.SetState(90)
				p.SequenceParams()
			}

		}

	case EqlBaseParserWITH:
		{
			p.SetState(93)
			p.SequenceParams()
		}
		p.SetState(95)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == EqlBaseParserBY {
			{
				p.SetState(94)

				var _x = p.JoinKeys()

				localctx.(*SequenceContext).disallowed = _x
			}

		}

	case EqlBaseParserLB, EqlBaseParserMISSING_EVENT_OPEN:

	default:
	}
	p.SetState(100)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == EqlBaseParserLB || _la == EqlBaseParserMISSING_EVENT_OPEN {
		{
			p.SetState(99)
			p.SequenceTerm()
		}

		p.SetState(102)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}
	p.SetState(106)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == EqlBaseParserUNTIL {
		{
			p.SetState(104)
			p.Match(EqlBaseParserUNTIL)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(105)

			var _x = p.SequenceTerm()

			localctx.(*SequenceContext).until = _x
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// ISampleContext is an interface to support dynamic dispatch.
type ISampleContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetBy returns the by rule contexts.
	GetBy() IJoinKeysContext

	// SetBy sets the by rule contexts.
	SetBy(IJoinKeysContext)

	// Getter signatures
	SAMPLE() antlr.TerminalNode
	AllJoinTerm() []IJoinTermContext
	JoinTerm(i int) IJoinTermContext
	JoinKeys() IJoinKeysContext

	// IsSampleContext differentiates from other interfaces.
	IsSampleContext()
}

type SampleContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	by     IJoinKeysContext
}

func NewEmptySampleContext() *SampleContext {
	var p = new(SampleContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_sample
	return p
}

func InitEmptySampleContext(p *SampleContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_sample
}

func (*SampleContext) IsSampleContext() {}

func NewSampleContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SampleContext {
	var p = new(SampleContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_sample

	return p
}

func (s *SampleContext) GetParser() antlr.Parser { return s.parser }

func (s *SampleContext) GetBy() IJoinKeysContext { return s.by }

func (s *SampleContext) SetBy(v IJoinKeysContext) { s.by = v }

func (s *SampleContext) SAMPLE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserSAMPLE, 0)
}

func (s *SampleContext) AllJoinTerm() []IJoinTermContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IJoinTermContext); ok {
			len++
		}
	}

	tst := make([]IJoinTermContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IJoinTermContext); ok {
			tst[i] = t.(IJoinTermContext)
			i++
		}
	}

	return tst
}

func (s *SampleContext) JoinTerm(i int) IJoinTermContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IJoinTermContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IJoinTermContext)
}

func (s *SampleContext) JoinKeys() IJoinKeysContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IJoinKeysContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IJoinKeysContext)
}

func (s *SampleContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SampleContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SampleContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterSample(s)
	}
}

func (s *SampleContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitSample(s)
	}
}

func (p *EqlBaseParser) Sample() (localctx ISampleContext) {
	localctx = NewSampleContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, EqlBaseParserRULE_sample)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(108)
		p.Match(EqlBaseParserSAMPLE)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(110)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == EqlBaseParserBY {
		{
			p.SetState(109)

			var _x = p.JoinKeys()

			localctx.(*SampleContext).by = _x
		}

	}
	p.SetState(113)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == EqlBaseParserLB || _la == EqlBaseParserMISSING_EVENT_OPEN {
		{
			p.SetState(112)
			p.JoinTerm()
		}

		p.SetState(115)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IJoinContext is an interface to support dynamic dispatch.
type IJoinContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetBy returns the by rule contexts.
	GetBy() IJoinKeysContext

	// GetUntil returns the until rule contexts.
	GetUntil() IJoinTermContext

	// SetBy sets the by rule contexts.
	SetBy(IJoinKeysContext)

	// SetUntil sets the until rule contexts.
	SetUntil(IJoinTermContext)

	// Getter signatures
	JOIN() antlr.TerminalNode
	AllJoinTerm() []IJoinTermContext
	JoinTerm(i int) IJoinTermContext
	UNTIL() antlr.TerminalNode
	JoinKeys() IJoinKeysContext

	// IsJoinContext differentiates from other interfaces.
	IsJoinContext()
}

type JoinContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	by     IJoinKeysContext
	until  IJoinTermContext
}

func NewEmptyJoinContext() *JoinContext {
	var p = new(JoinContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_join
	return p
}

func InitEmptyJoinContext(p *JoinContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_join
}

func (*JoinContext) IsJoinContext() {}

func NewJoinContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *JoinContext {
	var p = new(JoinContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_join

	return p
}

func (s *JoinContext) GetParser() antlr.Parser { return s.parser }

func (s *JoinContext) GetBy() IJoinKeysContext { return s.by }

func (s *JoinContext) GetUntil() IJoinTermContext { return s.until }

func (s *JoinContext) SetBy(v IJoinKeysContext) { s.by = v }

func (s *JoinContext) SetUntil(v IJoinTermContext) { s.until = v }

func (s *JoinContext) JOIN() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserJOIN, 0)
}

func (s *JoinContext) AllJoinTerm() []IJoinTermContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IJoinTermContext); ok {
			len++
		}
	}

	tst := make([]IJoinTermContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IJoinTermContext); ok {
			tst[i] = t.(IJoinTermContext)
			i++
		}
	}

	return tst
}

func (s *JoinContext) JoinTerm(i int) IJoinTermContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IJoinTermContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IJoinTermContext)
}

func (s *JoinContext) UNTIL() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserUNTIL, 0)
}

func (s *JoinContext) JoinKeys() IJoinKeysContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IJoinKeysContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IJoinKeysContext)
}

func (s *JoinContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *JoinContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *JoinContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterJoin(s)
	}
}

func (s *JoinContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitJoin(s)
	}
}

func (p *EqlBaseParser) Join() (localctx IJoinContext) {
	localctx = NewJoinContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 14, EqlBaseParserRULE_join)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(117)
		p.Match(EqlBaseParserJOIN)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(119)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == EqlBaseParserBY {
		{
			p.SetState(118)

			var _x = p.JoinKeys()

			localctx.(*JoinContext).by = _x
		}

	}
	{
		p.SetState(121)
		p.JoinTerm()
	}
	p.SetState(123)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == EqlBaseParserLB || _la == EqlBaseParserMISSING_EVENT_OPEN {
		{
			p.SetState(122)
			p.JoinTerm()
		}

		p.SetState(125)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}
	p.SetState(129)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == EqlBaseParserUNTIL {
		{
			p.SetState(127)
			p.Match(EqlBaseParserUNTIL)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(128)

			var _x = p.JoinTerm()

			localctx.(*JoinContext).until = _x
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IPipeContext is an interface to support dynamic dispatch.
type IPipeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetKind returns the kind token.
	GetKind() antlr.Token

	// SetKind sets the kind token.
	SetKind(antlr.Token)

	// Getter signatures
	PIPE() antlr.TerminalNode
	IDENTIFIER() antlr.TerminalNode
	AllBooleanExpression() []IBooleanExpressionContext
	BooleanExpression(i int) IBooleanExpressionContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsPipeContext differentiates from other interfaces.
	IsPipeContext()
}

type PipeContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	kind   antlr.Token
}

func NewEmptyPipeContext() *PipeContext {
	var p = new(PipeContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_pipe
	return p
}

func InitEmptyPipeContext(p *PipeContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_pipe
}

func (*PipeContext) IsPipeContext() {}

func NewPipeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *PipeContext {
	var p = new(PipeContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_pipe

	return p
}

func (s *PipeContext) GetParser() antlr.Parser { return s.parser }

func (s *PipeContext) GetKind() antlr.Token { return s.kind }

func (s *PipeContext) SetKind(v antlr.Token) { s.kind = v }

func (s *PipeContext) PIPE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserPIPE, 0)
}

func (s *PipeContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserIDENTIFIER, 0)
}

func (s *PipeContext) AllBooleanExpression() []IBooleanExpressionContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IBooleanExpressionContext); ok {
			len++
		}
	}

	tst := make([]IBooleanExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IBooleanExpressionContext); ok {
			tst[i] = t.(IBooleanExpressionContext)
			i++
		}
	}

	return tst
}

func (s *PipeContext) BooleanExpression(i int) IBooleanExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IBooleanExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IBooleanExpressionContext)
}

func (s *PipeContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(EqlBaseParserCOMMA)
}

func (s *PipeContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(EqlBaseParserCOMMA, i)
}

func (s *PipeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *PipeContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *PipeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterPipe(s)
	}
}

func (s *PipeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitPipe(s)
	}
}

func (p *EqlBaseParser) Pipe() (localctx IPipeContext) {
	localctx = NewPipeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 16, EqlBaseParserRULE_pipe)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(131)
		p.Match(EqlBaseParserPIPE)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(132)

		var _m = p.Match(EqlBaseParserIDENTIFIER)

		localctx.(*PipeContext).kind = _m
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(141)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&2226517489227792) != 0 {
		{
			p.SetState(133)
			p.booleanExpression(0)
		}
		p.SetState(138)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for _la == EqlBaseParserCOMMA {
			{
				p.SetState(134)
				p.Match(EqlBaseParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(135)
				p.booleanExpression(0)
			}

			p.SetState(140)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IJoinKeysContext is an interface to support dynamic dispatch.
type IJoinKeysContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	BY() antlr.TerminalNode
	AllExpression() []IExpressionContext
	Expression(i int) IExpressionContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsJoinKeysContext differentiates from other interfaces.
	IsJoinKeysContext()
}

type JoinKeysContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyJoinKeysContext() *JoinKeysContext {
	var p = new(JoinKeysContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_joinKeys
	return p
}

func InitEmptyJoinKeysContext(p *JoinKeysContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_joinKeys
}

func (*JoinKeysContext) IsJoinKeysContext() {}

func NewJoinKeysContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *JoinKeysContext {
	var p = new(JoinKeysContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_joinKeys

	return p
}

func (s *JoinKeysContext) GetParser() antlr.Parser { return s.parser }

func (s *JoinKeysContext) BY() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserBY, 0)
}

func (s *JoinKeysContext) AllExpression() []IExpressionContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExpressionContext); ok {
			len++
		}
	}

	tst := make([]IExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExpressionContext); ok {
			tst[i] = t.(IExpressionContext)
			i++
		}
	}

	return tst
}

func (s *JoinKeysContext) Expression(i int) IExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *JoinKeysContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(EqlBaseParserCOMMA)
}

func (s *JoinKeysContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(EqlBaseParserCOMMA, i)
}

func (s *JoinKeysContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *JoinKeysContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *JoinKeysContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterJoinKeys(s)
	}
}

func (s *JoinKeysContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitJoinKeys(s)
	}
}

func (p *EqlBaseParser) JoinKeys() (localctx IJoinKeysContext) {
	localctx = NewJoinKeysContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 18, EqlBaseParserRULE_joinKeys)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(143)
		p.Match(EqlBaseParserBY)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(144)
		p.Expression()
	}
	p.SetState(149)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == EqlBaseParserCOMMA {
		{
			p.SetState(145)
			p.Match(EqlBaseParserCOMMA)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(146)
			p.Expression()
		}

		p.SetState(151)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IJoinTermContext is an interface to support dynamic dispatch.
type IJoinTermContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetBy returns the by rule contexts.
	GetBy() IJoinKeysContext

	// SetBy sets the by rule contexts.
	SetBy(IJoinKeysContext)

	// Getter signatures
	Subquery() ISubqueryContext
	JoinKeys() IJoinKeysContext

	// IsJoinTermContext differentiates from other interfaces.
	IsJoinTermContext()
}

type JoinTermContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	by     IJoinKeysContext
}

func NewEmptyJoinTermContext() *JoinTermContext {
	var p = new(JoinTermContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_joinTerm
	return p
}

func InitEmptyJoinTermContext(p *JoinTermContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_joinTerm
}

func (*JoinTermContext) IsJoinTermContext() {}

func NewJoinTermContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *JoinTermContext {
	var p = new(JoinTermContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_joinTerm

	return p
}

func (s *JoinTermContext) GetParser() antlr.Parser { return s.parser }

func (s *JoinTermContext) GetBy() IJoinKeysContext { return s.by }

func (s *JoinTermContext) SetBy(v IJoinKeysContext) { s.by = v }

func (s *JoinTermContext) Subquery() ISubqueryContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISubqueryContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISubqueryContext)
}

func (s *JoinTermContext) JoinKeys() IJoinKeysContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IJoinKeysContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IJoinKeysContext)
}

func (s *JoinTermContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *JoinTermContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *JoinTermContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterJoinTerm(s)
	}
}

func (s *JoinTermContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitJoinTerm(s)
	}
}

func (p *EqlBaseParser) JoinTerm() (localctx IJoinTermContext) {
	localctx = NewJoinTermContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 20, EqlBaseParserRULE_joinTerm)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(152)
		p.Subquery()
	}
	p.SetState(154)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == EqlBaseParserBY {
		{
			p.SetState(153)

			var _x = p.JoinKeys()

			localctx.(*JoinTermContext).by = _x
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// ISequenceTermContext is an interface to support dynamic dispatch.
type ISequenceTermContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetKey returns the key token.
	GetKey() antlr.Token

	// SetKey sets the key token.
	SetKey(antlr.Token)

	// GetBy returns the by rule contexts.
	GetBy() IJoinKeysContext

	// GetValue returns the value rule contexts.
	GetValue() INumberContext

	// SetBy sets the by rule contexts.
	SetBy(IJoinKeysContext)

	// SetValue sets the value rule contexts.
	SetValue(INumberContext)

	// Getter signatures
	Subquery() ISubqueryContext
	WITH() antlr.TerminalNode
	ASGN() antlr.TerminalNode
	JoinKeys() IJoinKeysContext
	IDENTIFIER() antlr.TerminalNode
	Number() INumberContext

	// IsSequenceTermContext differentiates from other interfaces.
	IsSequenceTermContext()
}

type SequenceTermContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	by     IJoinKeysContext
	key    antlr.Token
	value  INumberContext
}

func NewEmptySequenceTermContext() *SequenceTermContext {
	var p = new(SequenceTermContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_sequenceTerm
	return p
}

func InitEmptySequenceTermContext(p *SequenceTermContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_sequenceTerm
}

func (*SequenceTermContext) IsSequenceTermContext() {}

func NewSequenceTermContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SequenceTermContext {
	var p = new(SequenceTermContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_sequenceTerm

	return p
}

func (s *SequenceTermContext) GetParser() antlr.Parser { return s.parser }

func (s *SequenceTermContext) GetKey() antlr.Token { return s.key }

func (s *SequenceTermContext) SetKey(v antlr.Token) { s.key = v }

func (s *SequenceTermContext) GetBy() IJoinKeysContext { return s.by }

func (s *SequenceTermContext) GetValue() INumberContext { return s.value }

func (s *SequenceTermContext) SetBy(v IJoinKeysContext) { s.by = v }

func (s *SequenceTermContext) SetValue(v INumberContext) { s.value = v }

func (s *SequenceTermContext) Subquery() ISubqueryContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISubqueryContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISubqueryContext)
}

func (s *SequenceTermContext) WITH() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserWITH, 0)
}

func (s *SequenceTermContext) ASGN() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserASGN, 0)
}

func (s *SequenceTermContext) JoinKeys() IJoinKeysContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IJoinKeysContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IJoinKeysContext)
}

func (s *SequenceTermContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserIDENTIFIER, 0)
}

func (s *SequenceTermContext) Number() INumberContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(INumberContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(INumberContext)
}

func (s *SequenceTermContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SequenceTermContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SequenceTermContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterSequenceTerm(s)
	}
}

func (s *SequenceTermContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitSequenceTerm(s)
	}
}

func (p *EqlBaseParser) SequenceTerm() (localctx ISequenceTermContext) {
	localctx = NewSequenceTermContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 22, EqlBaseParserRULE_sequenceTerm)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(156)
		p.Subquery()
	}
	p.SetState(158)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == EqlBaseParserBY {
		{
			p.SetState(157)

			var _x = p.JoinKeys()

			localctx.(*SequenceTermContext).by = _x
		}

	}
	p.SetState(164)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == EqlBaseParserWITH {
		{
			p.SetState(160)
			p.Match(EqlBaseParserWITH)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(161)

			var _m = p.Match(EqlBaseParserIDENTIFIER)

			localctx.(*SequenceTermContext).key = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(162)
			p.Match(EqlBaseParserASGN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(163)

			var _x = p.Number()

			localctx.(*SequenceTermContext).value = _x
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// ISubqueryContext is an interface to support dynamic dispatch.
type ISubqueryContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	EventFilter() IEventFilterContext
	RB() antlr.TerminalNode
	LB() antlr.TerminalNode
	MISSING_EVENT_OPEN() antlr.TerminalNode

	// IsSubqueryContext differentiates from other interfaces.
	IsSubqueryContext()
}

type SubqueryContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySubqueryContext() *SubqueryContext {
	var p = new(SubqueryContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_subquery
	return p
}

func InitEmptySubqueryContext(p *SubqueryContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_subquery
}

func (*SubqueryContext) IsSubqueryContext() {}

func NewSubqueryContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SubqueryContext {
	var p = new(SubqueryContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_subquery

	return p
}

func (s *SubqueryContext) GetParser() antlr.Parser { return s.parser }

func (s *SubqueryContext) EventFilter() IEventFilterContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEventFilterContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEventFilterContext)
}

func (s *SubqueryContext) RB() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserRB, 0)
}

func (s *SubqueryContext) LB() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserLB, 0)
}

func (s *SubqueryContext) MISSING_EVENT_OPEN() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserMISSING_EVENT_OPEN, 0)
}

func (s *SubqueryContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SubqueryContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SubqueryContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterSubquery(s)
	}
}

func (s *SubqueryContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitSubquery(s)
	}
}

func (p *EqlBaseParser) Subquery() (localctx ISubqueryContext) {
	localctx = NewSubqueryContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 24, EqlBaseParserRULE_subquery)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(166)
		_la = p.GetTokenStream().LA(1)

		if !(_la == EqlBaseParserLB || _la == EqlBaseParserMISSING_EVENT_OPEN) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}
	{
		p.SetState(167)
		p.EventFilter()
	}
	{
		p.SetState(168)
		p.Match(EqlBaseParserRB)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IEventQueryContext is an interface to support dynamic dispatch.
type IEventQueryContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	EventFilter() IEventFilterContext

	// IsEventQueryContext differentiates from other interfaces.
	IsEventQueryContext()
}

type EventQueryContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyEventQueryContext() *EventQueryContext {
	var p = new(EventQueryContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_eventQuery
	return p
}

func InitEmptyEventQueryContext(p *EventQueryContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_eventQuery
}

func (*EventQueryContext) IsEventQueryContext() {}

func NewEventQueryContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *EventQueryContext {
	var p = new(EventQueryContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_eventQuery

	return p
}

func (s *EventQueryContext) GetParser() antlr.Parser { return s.parser }

func (s *EventQueryContext) EventFilter() IEventFilterContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEventFilterContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEventFilterContext)
}

func (s *EventQueryContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *EventQueryContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *EventQueryContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterEventQuery(s)
	}
}

func (s *EventQueryContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitEventQuery(s)
	}
}

func (p *EqlBaseParser) EventQuery() (localctx IEventQueryContext) {
	localctx = NewEventQueryContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 26, EqlBaseParserRULE_eventQuery)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(170)
		p.EventFilter()
	}

	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IEventFilterContext is an interface to support dynamic dispatch.
type IEventFilterContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetEvent returns the event rule contexts.
	GetEvent() IEventValueContext

	// SetEvent sets the event rule contexts.
	SetEvent(IEventValueContext)

	// Getter signatures
	WHERE() antlr.TerminalNode
	Expression() IExpressionContext
	ANY() antlr.TerminalNode
	EventValue() IEventValueContext

	// IsEventFilterContext differentiates from other interfaces.
	IsEventFilterContext()
}

type EventFilterContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	event  IEventValueContext
}

func NewEmptyEventFilterContext() *EventFilterContext {
	var p = new(EventFilterContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_eventFilter
	return p
}

func InitEmptyEventFilterContext(p *EventFilterContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_eventFilter
}

func (*EventFilterContext) IsEventFilterContext() {}

func NewEventFilterContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *EventFilterContext {
	var p = new(EventFilterContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_eventFilter

	return p
}

func (s *EventFilterContext) GetParser() antlr.Parser { return s.parser }

func (s *EventFilterContext) GetEvent() IEventValueContext { return s.event }

func (s *EventFilterContext) SetEvent(v IEventValueContext) { s.event = v }

func (s *EventFilterContext) WHERE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserWHERE, 0)
}

func (s *EventFilterContext) Expression() IExpressionContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *EventFilterContext) ANY() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserANY, 0)
}

func (s *EventFilterContext) EventValue() IEventValueContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEventValueContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEventValueContext)
}

func (s *EventFilterContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *EventFilterContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *EventFilterContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterEventFilter(s)
	}
}

func (s *EventFilterContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitEventFilter(s)
	}
}

func (p *EqlBaseParser) EventFilter() (localctx IEventFilterContext) {
	localctx = NewEventFilterContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 28, EqlBaseParserRULE_eventFilter)
	p.EnterOuterAlt(localctx, 1)
	p.SetState(174)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case EqlBaseParserANY:
		{
			p.SetState(172)
			p.Match(EqlBaseParserANY)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case EqlBaseParserSTRING, EqlBaseParserIDENTIFIER:
		{
			p.SetState(173)

			var _x = p.EventValue()

			localctx.(*EventFilterContext).event = _x
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}
	{
		p.SetState(176)
		p.Match(EqlBaseParserWHERE)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(177)
		p.Expression()
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IExpressionContext is an interface to support dynamic dispatch.
type IExpressionContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	BooleanExpression() IBooleanExpressionContext

	// IsExpressionContext differentiates from other interfaces.
	IsExpressionContext()
}

type ExpressionContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyExpressionContext() *ExpressionContext {
	var p = new(ExpressionContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_expression
	return p
}

func InitEmptyExpressionContext(p *ExpressionContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_expression
}

func (*ExpressionContext) IsExpressionContext() {}

func NewExpressionContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ExpressionContext {
	var p = new(ExpressionContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_expression

	return p
}

func (s *ExpressionContext) GetParser() antlr.Parser { return s.parser }

func (s *ExpressionContext) BooleanExpression() IBooleanExpressionContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IBooleanExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IBooleanExpressionContext)
}

func (s *ExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ExpressionContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ExpressionContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterExpression(s)
	}
}

func (s *ExpressionContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitExpression(s)
	}
}

func (p *EqlBaseParser) Expression() (localctx IExpressionContext) {
	localctx = NewExpressionContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 30, EqlBaseParserRULE_expression)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(179)
		p.booleanExpression(0)
	}

	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IBooleanExpressionContext is an interface to support dynamic dispatch.
type IBooleanExpressionContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsBooleanExpressionContext differentiates from other interfaces.
	IsBooleanExpressionContext()
}

type BooleanExpressionContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyBooleanExpressionContext() *BooleanExpressionContext {
	var p = new(BooleanExpressionContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_booleanExpression
	return p
}

func InitEmptyBooleanExpressionContext(p *BooleanExpressionContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_booleanExpression
}

func (*BooleanExpressionContext) IsBooleanExpressionContext() {}

func NewBooleanExpressionContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *BooleanExpressionContext {
	var p = new(BooleanExpressionContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_booleanExpression

	return p
}

func (s *BooleanExpressionContext) GetParser() antlr.Parser { return s.parser }

func (s *BooleanExpressionContext) CopyAll(ctx *BooleanExpressionContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *BooleanExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BooleanExpressionContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type LogicalNotContext struct {
	BooleanExpressionContext
}

func NewLogicalNotContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *LogicalNotContext {
	var p = new(LogicalNotContext)

	InitEmptyBooleanExpressionContext(&p.BooleanExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*BooleanExpressionContext))

	return p
}

func (s *LogicalNotContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *LogicalNotContext) NOT() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserNOT, 0)
}

func (s *LogicalNotContext) BooleanExpression() IBooleanExpressionContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IBooleanExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IBooleanExpressionContext)
}

func (s *LogicalNotContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterLogicalNot(s)
	}
}

func (s *LogicalNotContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitLogicalNot(s)
	}
}

type BooleanDefaultContext struct {
	BooleanExpressionContext
}

func NewBooleanDefaultContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *BooleanDefaultContext {
	var p = new(BooleanDefaultContext)

	InitEmptyBooleanExpressionContext(&p.BooleanExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*BooleanExpressionContext))

	return p
}

func (s *BooleanDefaultContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BooleanDefaultContext) ValueExpression() IValueExpressionContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IValueExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IValueExpressionContext)
}

func (s *BooleanDefaultContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterBooleanDefault(s)
	}
}

func (s *BooleanDefaultContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitBooleanDefault(s)
	}
}

type ProcessCheckContext struct {
	BooleanExpressionContext
	relationship antlr.Token
}

func NewProcessCheckContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ProcessCheckContext {
	var p = new(ProcessCheckContext)

	InitEmptyBooleanExpressionContext(&p.BooleanExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*BooleanExpressionContext))

	return p
}

func (s *ProcessCheckContext) GetRelationship() antlr.Token { return s.relationship }

func (s *ProcessCheckContext) SetRelationship(v antlr.Token) { s.relationship = v }

func (s *ProcessCheckContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ProcessCheckContext) OF() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserOF, 0)
}

func (s *ProcessCheckContext) Subquery() ISubqueryContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISubqueryContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISubqueryContext)
}

func (s *ProcessCheckContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserIDENTIFIER, 0)
}

func (s *ProcessCheckContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterProcessCheck(s)
	}
}

func (s *ProcessCheckContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitProcessCheck(s)
	}
}

type LogicalBinaryContext struct {
	BooleanExpressionContext
	left     IBooleanExpressionContext
	operator antlr.Token
	right    IBooleanExpressionContext
}

func NewLogicalBinaryContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *LogicalBinaryContext {
	var p = new(LogicalBinaryContext)

	InitEmptyBooleanExpressionContext(&p.BooleanExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*BooleanExpressionContext))

	return p
}

func (s *LogicalBinaryContext) GetOperator() antlr.Token { return s.operator }

func (s *LogicalBinaryContext) SetOperator(v antlr.Token) { s.operator = v }

func (s *LogicalBinaryContext) GetLeft() IBooleanExpressionContext { return s.left }

func (s *LogicalBinaryContext) GetRight() IBooleanExpressionContext { return s.right }

func (s *LogicalBinaryContext) SetLeft(v IBooleanExpressionContext) { s.left = v }

func (s *LogicalBinaryContext) SetRight(v IBooleanExpressionContext) { s.right = v }

func (s *LogicalBinaryContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *LogicalBinaryContext) AllBooleanExpression() []IBooleanExpressionContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IBooleanExpressionContext); ok {
			len++
		}
	}

	tst := make([]IBooleanExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IBooleanExpressionContext); ok {
			tst[i] = t.(IBooleanExpressionContext)
			i++
		}
	}

	return tst
}

func (s *LogicalBinaryContext) BooleanExpression(i int) IBooleanExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IBooleanExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IBooleanExpressionContext)
}

func (s *LogicalBinaryContext) AND() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserAND, 0)
}

func (s *LogicalBinaryContext) OR() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserOR, 0)
}

func (s *LogicalBinaryContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterLogicalBinary(s)
	}
}

func (s *LogicalBinaryContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitLogicalBinary(s)
	}
}

func (p *EqlBaseParser) BooleanExpression() (localctx IBooleanExpressionContext) {
	return p.booleanExpression(0)
}

func (p *EqlBaseParser) booleanExpression(_p int) (localctx IBooleanExpressionContext) {
	var _parentctx antlr.ParserRuleContext = p.GetParserRuleContext()

	_parentState := p.GetState()
	localctx = NewBooleanExpressionContext(p, p.GetParserRuleContext(), _parentState)
	var _prevctx IBooleanExpressionContext = localctx
	var _ antlr.ParserRuleContext = _prevctx // TODO: To prevent unused variable warning.
	_startState := 32
	p.EnterRecursionRule(localctx, 32, EqlBaseParserRULE_booleanExpression, _p)
	var _alt int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(188)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 19, p.GetParserRuleContext()) {
	case 1:
		localctx = NewLogicalNotContext(p, localctx)
		p.SetParserRuleContext(localctx)
		_prevctx = localctx

		{
			p.SetState(182)
			p.Match(EqlBaseParserNOT)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(183)
			p.booleanExpression(5)
		}

	case 2:
		localctx = NewProcessCheckContext(p, localctx)
		p.SetParserRuleContext(localctx)
		_prevctx = localctx
		{
			p.SetState(184)

			var _m = p.Match(EqlBaseParserIDENTIFIER)

			localctx.(*ProcessCheckContext).relationship = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(185)
			p.Match(EqlBaseParserOF)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(186)
			p.Subquery()
		}

	case 3:
		localctx = NewBooleanDefaultContext(p, localctx)
		p.SetParserRuleContext(localctx)
		_prevctx = localctx
		{
			p.SetState(187)
			p.ValueExpression()
		}

	case antlr.ATNInvalidAltNumber:
		goto errorExit
	}
	p.GetParserRuleContext().SetStop(p.GetTokenStream().LT(-1))
	p.SetState(198)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 21, p.GetParserRuleContext())
	if p.HasError() {
		goto errorExit
	}
	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			if p.GetParseListeners() != nil {
				p.TriggerExitRuleEvent()
			}
			_prevctx = localctx
			p.SetState(196)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}

			switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 20, p.GetParserRuleContext()) {
			case 1:
				localctx = NewLogicalBinaryContext(p, NewBooleanExpressionContext(p, _parentctx, _parentState))
				localctx.(*LogicalBinaryContext).left = _prevctx

				p.PushNewRecursionContext(localctx, _startState, EqlBaseParserRULE_booleanExpression)
				p.SetState(190)

				if !(p.Precpred(p.GetParserRuleContext(), 2)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 2)", ""))
					goto errorExit
				}
				{
					p.SetState(191)

					var _m = p.Match(EqlBaseParserAND)

					localctx.(*LogicalBinaryContext).operator = _m
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				{
					p.SetState(192)

					var _x = p.booleanExpression(3)

					localctx.(*LogicalBinaryContext).right = _x
				}

			case 2:
				localctx = NewLogicalBinaryContext(p, NewBooleanExpressionContext(p, _parentctx, _parentState))
				localctx.(*LogicalBinaryContext).left = _prevctx

				p.PushNewRecursionContext(localctx, _startState, EqlBaseParserRULE_booleanExpression)
				p.SetState(193)

				if !(p.Precpred(p.GetParserRuleContext(), 1)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 1)", ""))
					goto errorExit
				}
				{
					p.SetState(194)

					var _m = p.Match(EqlBaseParserOR)

					localctx.(*LogicalBinaryContext).operator = _m
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				{
					p.SetState(195)

					var _x = p.booleanExpression(2)

					localctx.(*LogicalBinaryContext).right = _x
				}

			case antlr.ATNInvalidAltNumber:
				goto errorExit
			}

		}
		p.SetState(200)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 21, p.GetParserRuleContext())
		if p.HasError() {
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.UnrollRecursionContexts(_parentctx)
	return localctx
}

// IValueExpressionContext is an interface to support dynamic dispatch.
type IValueExpressionContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsValueExpressionContext differentiates from other interfaces.
	IsValueExpressionContext()
}

type ValueExpressionContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyValueExpressionContext() *ValueExpressionContext {
	var p = new(ValueExpressionContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_valueExpression
	return p
}

func InitEmptyValueExpressionContext(p *ValueExpressionContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_valueExpression
}

func (*ValueExpressionContext) IsValueExpressionContext() {}

func NewValueExpressionContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ValueExpressionContext {
	var p = new(ValueExpressionContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_valueExpression

	return p
}

func (s *ValueExpressionContext) GetParser() antlr.Parser { return s.parser }

func (s *ValueExpressionContext) CopyAll(ctx *ValueExpressionContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *ValueExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ValueExpressionContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type ValueExpressionDefaultContext struct {
	ValueExpressionContext
}

func NewValueExpressionDefaultContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ValueExpressionDefaultContext {
	var p = new(ValueExpressionDefaultContext)

	InitEmptyValueExpressionContext(&p.ValueExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*ValueExpressionContext))

	return p
}

func (s *ValueExpressionDefaultContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ValueExpressionDefaultContext) OperatorExpression() IOperatorExpressionContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IOperatorExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IOperatorExpressionContext)
}

func (s *ValueExpressionDefaultContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterValueExpressionDefault(s)
	}
}

func (s *ValueExpressionDefaultContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitValueExpressionDefault(s)
	}
}

type ComparisonContext struct {
	ValueExpressionContext
	left  IOperatorExpressionContext
	right IOperatorExpressionContext
}

func NewComparisonContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ComparisonContext {
	var p = new(ComparisonContext)

	InitEmptyValueExpressionContext(&p.ValueExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*ValueExpressionContext))

	return p
}

func (s *ComparisonContext) GetLeft() IOperatorExpressionContext { return s.left }

func (s *ComparisonContext) GetRight() IOperatorExpressionContext { return s.right }

func (s *ComparisonContext) SetLeft(v IOperatorExpressionContext) { s.left = v }

func (s *ComparisonContext) SetRight(v IOperatorExpressionContext) { s.right = v }

func (s *ComparisonContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ComparisonContext) ComparisonOperator() IComparisonOperatorContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IComparisonOperatorContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IComparisonOperatorContext)
}

func (s *ComparisonContext) AllOperatorExpression() []IOperatorExpressionContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IOperatorExpressionContext); ok {
			len++
		}
	}

	tst := make([]IOperatorExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IOperatorExpressionContext); ok {
			tst[i] = t.(IOperatorExpressionContext)
			i++
		}
	}

	return tst
}

func (s *ComparisonContext) OperatorExpression(i int) IOperatorExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IOperatorExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IOperatorExpressionContext)
}

func (s *ComparisonContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterComparison(s)
	}
}

func (s *ComparisonContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitComparison(s)
	}
}

func (p *EqlBaseParser) ValueExpression() (localctx IValueExpressionContext) {
	localctx = NewValueExpressionContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 34, EqlBaseParserRULE_valueExpression)
	p.SetState(206)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 22, p.GetParserRuleContext()) {
	case 1:
		localctx = NewValueExpressionDefaultContext(p, localctx)
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(201)
			p.operatorExpression(0)
		}

	case 2:
		localctx = NewComparisonContext(p, localctx)
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(202)

			var _x = p.operatorExpression(0)

			localctx.(*ComparisonContext).left = _x
		}
		{
			p.SetState(203)
			p.ComparisonOperator()
		}
		{
			p.SetState(204)

			var _x = p.operatorExpression(0)

			localctx.(*ComparisonContext).right = _x
		}

	case antlr.ATNInvalidAltNumber:
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IOperatorExpressionContext is an interface to support dynamic dispatch.
type IOperatorExpressionContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsOperatorExpressionContext differentiates from other interfaces.
	IsOperatorExpressionContext()
}

type OperatorExpressionContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyOperatorExpressionContext() *OperatorExpressionContext {
	var p = new(OperatorExpressionContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_operatorExpression
	return p
}

func InitEmptyOperatorExpressionContext(p *OperatorExpressionContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_operatorExpression
}

func (*OperatorExpressionContext) IsOperatorExpressionContext() {}

func NewOperatorExpressionContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *OperatorExpressionContext {
	var p = new(OperatorExpressionContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_operatorExpression

	return p
}

func (s *OperatorExpressionContext) GetParser() antlr.Parser { return s.parser }

func (s *OperatorExpressionContext) CopyAll(ctx *OperatorExpressionContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *OperatorExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *OperatorExpressionContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type OperatorExpressionDefaultContext struct {
	OperatorExpressionContext
}

func NewOperatorExpressionDefaultContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *OperatorExpressionDefaultContext {
	var p = new(OperatorExpressionDefaultContext)

	InitEmptyOperatorExpressionContext(&p.OperatorExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*OperatorExpressionContext))

	return p
}

func (s *OperatorExpressionDefaultContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *OperatorExpressionDefaultContext) PrimaryExpression() IPrimaryExpressionContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IPrimaryExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IPrimaryExpressionContext)
}

func (s *OperatorExpressionDefaultContext) Predicate() IPredicateContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IPredicateContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IPredicateContext)
}

func (s *OperatorExpressionDefaultContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterOperatorExpressionDefault(s)
	}
}

func (s *OperatorExpressionDefaultContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitOperatorExpressionDefault(s)
	}
}

type ArithmeticBinaryContext struct {
	OperatorExpressionContext
	left     IOperatorExpressionContext
	operator antlr.Token
	right    IOperatorExpressionContext
}

func NewArithmeticBinaryContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ArithmeticBinaryContext {
	var p = new(ArithmeticBinaryContext)

	InitEmptyOperatorExpressionContext(&p.OperatorExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*OperatorExpressionContext))

	return p
}

func (s *ArithmeticBinaryContext) GetOperator() antlr.Token { return s.operator }

func (s *ArithmeticBinaryContext) SetOperator(v antlr.Token) { s.operator = v }

func (s *ArithmeticBinaryContext) GetLeft() IOperatorExpressionContext { return s.left }

func (s *ArithmeticBinaryContext) GetRight() IOperatorExpressionContext { return s.right }

func (s *ArithmeticBinaryContext) SetLeft(v IOperatorExpressionContext) { s.left = v }

func (s *ArithmeticBinaryContext) SetRight(v IOperatorExpressionContext) { s.right = v }

func (s *ArithmeticBinaryContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ArithmeticBinaryContext) AllOperatorExpression() []IOperatorExpressionContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IOperatorExpressionContext); ok {
			len++
		}
	}

	tst := make([]IOperatorExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IOperatorExpressionContext); ok {
			tst[i] = t.(IOperatorExpressionContext)
			i++
		}
	}

	return tst
}

func (s *ArithmeticBinaryContext) OperatorExpression(i int) IOperatorExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IOperatorExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IOperatorExpressionContext)
}

func (s *ArithmeticBinaryContext) ASTERISK() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserASTERISK, 0)
}

func (s *ArithmeticBinaryContext) SLASH() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserSLASH, 0)
}

func (s *ArithmeticBinaryContext) PERCENT() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserPERCENT, 0)
}

func (s *ArithmeticBinaryContext) PLUS() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserPLUS, 0)
}

func (s *ArithmeticBinaryContext) MINUS() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserMINUS, 0)
}

func (s *ArithmeticBinaryContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterArithmeticBinary(s)
	}
}

func (s *ArithmeticBinaryContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitArithmeticBinary(s)
	}
}

type ArithmeticUnaryContext struct {
	OperatorExpressionContext
	operator antlr.Token
}

func NewArithmeticUnaryContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ArithmeticUnaryContext {
	var p = new(ArithmeticUnaryContext)

	InitEmptyOperatorExpressionContext(&p.OperatorExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*OperatorExpressionContext))

	return p
}

func (s *ArithmeticUnaryContext) GetOperator() antlr.Token { return s.operator }

func (s *ArithmeticUnaryContext) SetOperator(v antlr.Token) { s.operator = v }

func (s *ArithmeticUnaryContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ArithmeticUnaryContext) OperatorExpression() IOperatorExpressionContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IOperatorExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IOperatorExpressionContext)
}

func (s *ArithmeticUnaryContext) MINUS() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserMINUS, 0)
}

func (s *ArithmeticUnaryContext) PLUS() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserPLUS, 0)
}

func (s *ArithmeticUnaryContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterArithmeticUnary(s)
	}
}

func (s *ArithmeticUnaryContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitArithmeticUnary(s)
	}
}

func (p *EqlBaseParser) OperatorExpression() (localctx IOperatorExpressionContext) {
	return p.operatorExpression(0)
}

func (p *EqlBaseParser) operatorExpression(_p int) (localctx IOperatorExpressionContext) {
	var _parentctx antlr.ParserRuleContext = p.GetParserRuleContext()

	_parentState := p.GetState()
	localctx = NewOperatorExpressionContext(p, p.GetParserRuleContext(), _parentState)
	var _prevctx IOperatorExpressionContext = localctx
	var _ antlr.ParserRuleContext = _prevctx // TODO: To prevent unused variable warning.
	_startState := 36
	p.EnterRecursionRule(localctx, 36, EqlBaseParserRULE_operatorExpression, _p)
	var _la int

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(215)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case EqlBaseParserFALSE, EqlBaseParserNULL, EqlBaseParserTRUE, EqlBaseParserLP, EqlBaseParserOPTIONAL, EqlBaseParserSTRING, EqlBaseParserINTEGER_VALUE, EqlBaseParserDECIMAL_VALUE, EqlBaseParserIDENTIFIER, EqlBaseParserQUOTED_IDENTIFIER, EqlBaseParserTILDE_IDENTIFIER:
		localctx = NewOperatorExpressionDefaultContext(p, localctx)
		p.SetParserRuleContext(localctx)
		_prevctx = localctx

		{
			p.SetState(209)
			p.PrimaryExpression()
		}
		p.SetState(211)
		p.GetErrorHandler().Sync(p)

		if p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 23, p.GetParserRuleContext()) == 1 {
			{
				p.SetState(210)
				p.Predicate()
			}

		} else if p.HasError() { // JIM
			goto errorExit
		}

	case EqlBaseParserPLUS, EqlBaseParserMINUS:
		localctx = NewArithmeticUnaryContext(p, localctx)
		p.SetParserRuleContext(localctx)
		_prevctx = localctx
		{
			p.SetState(213)

			var _lt = p.GetTokenStream().LT(1)

			localctx.(*ArithmeticUnaryContext).operator = _lt

			_la = p.GetTokenStream().LA(1)

			if !(_la == EqlBaseParserPLUS || _la == EqlBaseParserMINUS) {
				var _ri = p.GetErrorHandler().RecoverInline(p)

				localctx.(*ArithmeticUnaryContext).operator = _ri
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}
		{
			p.SetState(214)
			p.operatorExpression(3)
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}
	p.GetParserRuleContext().SetStop(p.GetTokenStream().LT(-1))
	p.SetState(225)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 26, p.GetParserRuleContext())
	if p.HasError() {
		goto errorExit
	}
	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			if p.GetParseListeners() != nil {
				p.TriggerExitRuleEvent()
			}
			_prevctx = localctx
			p.SetState(223)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}

			switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 25, p.GetParserRuleContext()) {
			case 1:
				localctx = NewArithmeticBinaryContext(p, NewOperatorExpressionContext(p, _parentctx, _parentState))
				localctx.(*ArithmeticBinaryContext).left = _prevctx

				p.PushNewRecursionContext(localctx, _startState, EqlBaseParserRULE_operatorExpression)
				p.SetState(217)

				if !(p.Precpred(p.GetParserRuleContext(), 2)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 2)", ""))
					goto errorExit
				}
				{
					p.SetState(218)

					var _lt = p.GetTokenStream().LT(1)

					localctx.(*ArithmeticBinaryContext).operator = _lt

					_la = p.GetTokenStream().LA(1)

					if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&60129542144) != 0) {
						var _ri = p.GetErrorHandler().RecoverInline(p)

						localctx.(*ArithmeticBinaryContext).operator = _ri
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(219)

					var _x = p.operatorExpression(3)

					localctx.(*ArithmeticBinaryContext).right = _x
				}

			case 2:
				localctx = NewArithmeticBinaryContext(p, NewOperatorExpressionContext(p, _parentctx, _parentState))
				localctx.(*ArithmeticBinaryContext).left = _prevctx

				p.PushNewRecursionContext(localctx, _startState, EqlBaseParserRULE_operatorExpression)
				p.SetState(220)

				if !(p.Precpred(p.GetParserRuleContext(), 1)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 1)", ""))
					goto errorExit
				}
				{
					p.SetState(221)

					var _lt = p.GetTokenStream().LT(1)

					localctx.(*ArithmeticBinaryContext).operator = _lt

					_la = p.GetTokenStream().LA(1)

					if !(_la == EqlBaseParserPLUS || _la == EqlBaseParserMINUS) {
						var _ri = p.GetErrorHandler().RecoverInline(p)

						localctx.(*ArithmeticBinaryContext).operator = _ri
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(222)

					var _x = p.operatorExpression(2)

					localctx.(*ArithmeticBinaryContext).right = _x
				}

			case antlr.ATNInvalidAltNumber:
				goto errorExit
			}

		}
		p.SetState(227)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 26, p.GetParserRuleContext())
		if p.HasError() {
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.UnrollRecursionContexts(_parentctx)
	return localctx
}

// IPredicateContext is an interface to support dynamic dispatch.
type IPredicateContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetKind returns the kind token.
	GetKind() antlr.Token

	// SetKind sets the kind token.
	SetKind(antlr.Token)

	// Getter signatures
	LP() antlr.TerminalNode
	AllExpression() []IExpressionContext
	Expression(i int) IExpressionContext
	RP() antlr.TerminalNode
	IN() antlr.TerminalNode
	IN_INSENSITIVE() antlr.TerminalNode
	NOT() antlr.TerminalNode
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode
	AllConstant() []IConstantContext
	Constant(i int) IConstantContext
	SEQ() antlr.TerminalNode
	LIKE() antlr.TerminalNode
	LIKE_INSENSITIVE() antlr.TerminalNode
	REGEX() antlr.TerminalNode
	REGEX_INSENSITIVE() antlr.TerminalNode

	// IsPredicateContext differentiates from other interfaces.
	IsPredicateContext()
}

type PredicateContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	kind   antlr.Token
}

func NewEmptyPredicateContext() *PredicateContext {
	var p = new(PredicateContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_predicate
	return p
}

func InitEmptyPredicateContext(p *PredicateContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_predicate
}

func (*PredicateContext) IsPredicateContext() {}

func NewPredicateContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *PredicateContext {
	var p = new(PredicateContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_predicate

	return p
}

func (s *PredicateContext) GetParser() antlr.Parser { return s.parser }

func (s *PredicateContext) GetKind() antlr.Token { return s.kind }

func (s *PredicateContext) SetKind(v antlr.Token) { s.kind = v }

func (s *PredicateContext) LP() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserLP, 0)
}

func (s *PredicateContext) AllExpression() []IExpressionContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExpressionContext); ok {
			len++
		}
	}

	tst := make([]IExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExpressionContext); ok {
			tst[i] = t.(IExpressionContext)
			i++
		}
	}

	return tst
}

func (s *PredicateContext) Expression(i int) IExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *PredicateContext) RP() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserRP, 0)
}

func (s *PredicateContext) IN() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserIN, 0)
}

func (s *PredicateContext) IN_INSENSITIVE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserIN_INSENSITIVE, 0)
}

func (s *PredicateContext) NOT() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserNOT, 0)
}

func (s *PredicateContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(EqlBaseParserCOMMA)
}

func (s *PredicateContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(EqlBaseParserCOMMA, i)
}

func (s *PredicateContext) AllConstant() []IConstantContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IConstantContext); ok {
			len++
		}
	}

	tst := make([]IConstantContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IConstantContext); ok {
			tst[i] = t.(IConstantContext)
			i++
		}
	}

	return tst
}

func (s *PredicateContext) Constant(i int) IConstantContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IConstantContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IConstantContext)
}

func (s *PredicateContext) SEQ() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserSEQ, 0)
}

func (s *PredicateContext) LIKE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserLIKE, 0)
}

func (s *PredicateContext) LIKE_INSENSITIVE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserLIKE_INSENSITIVE, 0)
}

func (s *PredicateContext) REGEX() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserREGEX, 0)
}

func (s *PredicateContext) REGEX_INSENSITIVE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserREGEX_INSENSITIVE, 0)
}

func (s *PredicateContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *PredicateContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *PredicateContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterPredicate(s)
	}
}

func (s *PredicateContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitPredicate(s)
	}
}

func (p *EqlBaseParser) Predicate() (localctx IPredicateContext) {
	localctx = NewPredicateContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 38, EqlBaseParserRULE_predicate)
	var _la int

	p.SetState(257)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 30, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		p.SetState(229)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == EqlBaseParserNOT {
			{
				p.SetState(228)
				p.Match(EqlBaseParserNOT)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		}
		{
			p.SetState(231)

			var _lt = p.GetTokenStream().LT(1)

			localctx.(*PredicateContext).kind = _lt

			_la = p.GetTokenStream().LA(1)

			if !(_la == EqlBaseParserIN || _la == EqlBaseParserIN_INSENSITIVE) {
				var _ri = p.GetErrorHandler().RecoverInline(p)

				localctx.(*PredicateContext).kind = _ri
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}
		{
			p.SetState(232)
			p.Match(EqlBaseParserLP)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(233)
			p.Expression()
		}
		p.SetState(238)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for _la == EqlBaseParserCOMMA {
			{
				p.SetState(234)
				p.Match(EqlBaseParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(235)
				p.Expression()
			}

			p.SetState(240)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}
		{
			p.SetState(241)
			p.Match(EqlBaseParserRP)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(243)

			var _lt = p.GetTokenStream().LT(1)

			localctx.(*PredicateContext).kind = _lt

			_la = p.GetTokenStream().LA(1)

			if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&8487680) != 0) {
				var _ri = p.GetErrorHandler().RecoverInline(p)

				localctx.(*PredicateContext).kind = _ri
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}
		{
			p.SetState(244)
			p.Constant()
		}

	case 3:
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(245)

			var _lt = p.GetTokenStream().LT(1)

			localctx.(*PredicateContext).kind = _lt

			_la = p.GetTokenStream().LA(1)

			if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&8487680) != 0) {
				var _ri = p.GetErrorHandler().RecoverInline(p)

				localctx.(*PredicateContext).kind = _ri
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}
		{
			p.SetState(246)
			p.Match(EqlBaseParserLP)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(247)
			p.Constant()
		}
		p.SetState(252)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for _la == EqlBaseParserCOMMA {
			{
				p.SetState(248)
				p.Match(EqlBaseParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(249)
				p.Constant()
			}

			p.SetState(254)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}
		{
			p.SetState(255)
			p.Match(EqlBaseParserRP)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case antlr.ATNInvalidAltNumber:
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IPrimaryExpressionContext is an interface to support dynamic dispatch.
type IPrimaryExpressionContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsPrimaryExpressionContext differentiates from other interfaces.
	IsPrimaryExpressionContext()
}

type PrimaryExpressionContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyPrimaryExpressionContext() *PrimaryExpressionContext {
	var p = new(PrimaryExpressionContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_primaryExpression
	return p
}

func InitEmptyPrimaryExpressionContext(p *PrimaryExpressionContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_primaryExpression
}

func (*PrimaryExpressionContext) IsPrimaryExpressionContext() {}

func NewPrimaryExpressionContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *PrimaryExpressionContext {
	var p = new(PrimaryExpressionContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_primaryExpression

	return p
}

func (s *PrimaryExpressionContext) GetParser() antlr.Parser { return s.parser }

func (s *PrimaryExpressionContext) CopyAll(ctx *PrimaryExpressionContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *PrimaryExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *PrimaryExpressionContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type DereferenceContext struct {
	PrimaryExpressionContext
}

func NewDereferenceContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *DereferenceContext {
	var p = new(DereferenceContext)

	InitEmptyPrimaryExpressionContext(&p.PrimaryExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*PrimaryExpressionContext))

	return p
}

func (s *DereferenceContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DereferenceContext) QualifiedName() IQualifiedNameContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IQualifiedNameContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IQualifiedNameContext)
}

func (s *DereferenceContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterDereference(s)
	}
}

func (s *DereferenceContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitDereference(s)
	}
}

type ConstantDefaultContext struct {
	PrimaryExpressionContext
}

func NewConstantDefaultContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ConstantDefaultContext {
	var p = new(ConstantDefaultContext)

	InitEmptyPrimaryExpressionContext(&p.PrimaryExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*PrimaryExpressionContext))

	return p
}

func (s *ConstantDefaultContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ConstantDefaultContext) Constant() IConstantContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IConstantContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IConstantContext)
}

func (s *ConstantDefaultContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterConstantDefault(s)
	}
}

func (s *ConstantDefaultContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitConstantDefault(s)
	}
}

type ParenthesizedExpressionContext struct {
	PrimaryExpressionContext
}

func NewParenthesizedExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ParenthesizedExpressionContext {
	var p = new(ParenthesizedExpressionContext)

	InitEmptyPrimaryExpressionContext(&p.PrimaryExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*PrimaryExpressionContext))

	return p
}

func (s *ParenthesizedExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ParenthesizedExpressionContext) LP() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserLP, 0)
}

func (s *ParenthesizedExpressionContext) Expression() IExpressionContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *ParenthesizedExpressionContext) RP() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserRP, 0)
}

func (s *ParenthesizedExpressionContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterParenthesizedExpression(s)
	}
}

func (s *ParenthesizedExpressionContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitParenthesizedExpression(s)
	}
}

type FunctionContext struct {
	PrimaryExpressionContext
}

func NewFunctionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *FunctionContext {
	var p = new(FunctionContext)

	InitEmptyPrimaryExpressionContext(&p.PrimaryExpressionContext)
	p.parser = parser
	p.CopyAll(ctx.(*PrimaryExpressionContext))

	return p
}

func (s *FunctionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FunctionContext) FunctionExpression() IFunctionExpressionContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFunctionExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFunctionExpressionContext)
}

func (s *FunctionContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterFunction(s)
	}
}

func (s *FunctionContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitFunction(s)
	}
}

func (p *EqlBaseParser) PrimaryExpression() (localctx IPrimaryExpressionContext) {
	localctx = NewPrimaryExpressionContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 40, EqlBaseParserRULE_primaryExpression)
	p.SetState(266)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 31, p.GetParserRuleContext()) {
	case 1:
		localctx = NewConstantDefaultContext(p, localctx)
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(259)
			p.Constant()
		}

	case 2:
		localctx = NewFunctionContext(p, localctx)
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(260)
			p.FunctionExpression()
		}

	case 3:
		localctx = NewDereferenceContext(p, localctx)
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(261)
			p.QualifiedName()
		}

	case 4:
		localctx = NewParenthesizedExpressionContext(p, localctx)
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(262)
			p.Match(EqlBaseParserLP)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(263)
			p.Expression()
		}
		{
			p.SetState(264)
			p.Match(EqlBaseParserRP)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case antlr.ATNInvalidAltNumber:
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IFunctionExpressionContext is an interface to support dynamic dispatch.
type IFunctionExpressionContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetName returns the name rule contexts.
	GetName() IFunctionNameContext

	// SetName sets the name rule contexts.
	SetName(IFunctionNameContext)

	// Getter signatures
	LP() antlr.TerminalNode
	RP() antlr.TerminalNode
	FunctionName() IFunctionNameContext
	AllExpression() []IExpressionContext
	Expression(i int) IExpressionContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsFunctionExpressionContext differentiates from other interfaces.
	IsFunctionExpressionContext()
}

type FunctionExpressionContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	name   IFunctionNameContext
}

func NewEmptyFunctionExpressionContext() *FunctionExpressionContext {
	var p = new(FunctionExpressionContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_functionExpression
	return p
}

func InitEmptyFunctionExpressionContext(p *FunctionExpressionContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_functionExpression
}

func (*FunctionExpressionContext) IsFunctionExpressionContext() {}

func NewFunctionExpressionContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FunctionExpressionContext {
	var p = new(FunctionExpressionContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_functionExpression

	return p
}

func (s *FunctionExpressionContext) GetParser() antlr.Parser { return s.parser }

func (s *FunctionExpressionContext) GetName() IFunctionNameContext { return s.name }

func (s *FunctionExpressionContext) SetName(v IFunctionNameContext) { s.name = v }

func (s *FunctionExpressionContext) LP() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserLP, 0)
}

func (s *FunctionExpressionContext) RP() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserRP, 0)
}

func (s *FunctionExpressionContext) FunctionName() IFunctionNameContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFunctionNameContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFunctionNameContext)
}

func (s *FunctionExpressionContext) AllExpression() []IExpressionContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExpressionContext); ok {
			len++
		}
	}

	tst := make([]IExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExpressionContext); ok {
			tst[i] = t.(IExpressionContext)
			i++
		}
	}

	return tst
}

func (s *FunctionExpressionContext) Expression(i int) IExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *FunctionExpressionContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(EqlBaseParserCOMMA)
}

func (s *FunctionExpressionContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(EqlBaseParserCOMMA, i)
}

func (s *FunctionExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FunctionExpressionContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FunctionExpressionContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterFunctionExpression(s)
	}
}

func (s *FunctionExpressionContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitFunctionExpression(s)
	}
}

func (p *EqlBaseParser) FunctionExpression() (localctx IFunctionExpressionContext) {
	localctx = NewFunctionExpressionContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 42, EqlBaseParserRULE_functionExpression)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(268)

		var _x = p.FunctionName()

		localctx.(*FunctionExpressionContext).name = _x
	}
	{
		p.SetState(269)
		p.Match(EqlBaseParserLP)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(278)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&2226517489227792) != 0 {
		{
			p.SetState(270)
			p.Expression()
		}
		p.SetState(275)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for _la == EqlBaseParserCOMMA {
			{
				p.SetState(271)
				p.Match(EqlBaseParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(272)
				p.Expression()
			}

			p.SetState(277)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}

	}
	{
		p.SetState(280)
		p.Match(EqlBaseParserRP)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IFunctionNameContext is an interface to support dynamic dispatch.
type IFunctionNameContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	IDENTIFIER() antlr.TerminalNode
	TILDE_IDENTIFIER() antlr.TerminalNode

	// IsFunctionNameContext differentiates from other interfaces.
	IsFunctionNameContext()
}

type FunctionNameContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFunctionNameContext() *FunctionNameContext {
	var p = new(FunctionNameContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_functionName
	return p
}

func InitEmptyFunctionNameContext(p *FunctionNameContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_functionName
}

func (*FunctionNameContext) IsFunctionNameContext() {}

func NewFunctionNameContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FunctionNameContext {
	var p = new(FunctionNameContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_functionName

	return p
}

func (s *FunctionNameContext) GetParser() antlr.Parser { return s.parser }

func (s *FunctionNameContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserIDENTIFIER, 0)
}

func (s *FunctionNameContext) TILDE_IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserTILDE_IDENTIFIER, 0)
}

func (s *FunctionNameContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FunctionNameContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FunctionNameContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterFunctionName(s)
	}
}

func (s *FunctionNameContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitFunctionName(s)
	}
}

func (p *EqlBaseParser) FunctionName() (localctx IFunctionNameContext) {
	localctx = NewFunctionNameContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 44, EqlBaseParserRULE_functionName)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(282)
		_la = p.GetTokenStream().LA(1)

		if !(_la == EqlBaseParserIDENTIFIER || _la == EqlBaseParserTILDE_IDENTIFIER) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IConstantContext is an interface to support dynamic dispatch.
type IConstantContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsConstantContext differentiates from other interfaces.
	IsConstantContext()
}

type ConstantContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyConstantContext() *ConstantContext {
	var p = new(ConstantContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_constant
	return p
}

func InitEmptyConstantContext(p *ConstantContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_constant
}

func (*ConstantContext) IsConstantContext() {}

func NewConstantContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ConstantContext {
	var p = new(ConstantContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_constant

	return p
}

func (s *ConstantContext) GetParser() antlr.Parser { return s.parser }

func (s *ConstantContext) CopyAll(ctx *ConstantContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *ConstantContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ConstantContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type NullLiteralContext struct {
	ConstantContext
}

func NewNullLiteralContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *NullLiteralContext {
	var p = new(NullLiteralContext)

	InitEmptyConstantContext(&p.ConstantContext)
	p.parser = parser
	p.CopyAll(ctx.(*ConstantContext))

	return p
}

func (s *NullLiteralContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *NullLiteralContext) NULL() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserNULL, 0)
}

func (s *NullLiteralContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterNullLiteral(s)
	}
}

func (s *NullLiteralContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitNullLiteral(s)
	}
}

type StringLiteralContext struct {
	ConstantContext
}

func NewStringLiteralContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *StringLiteralContext {
	var p = new(StringLiteralContext)

	InitEmptyConstantContext(&p.ConstantContext)
	p.parser = parser
	p.CopyAll(ctx.(*ConstantContext))

	return p
}

func (s *StringLiteralContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *StringLiteralContext) String_() IStringContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IStringContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IStringContext)
}

func (s *StringLiteralContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterStringLiteral(s)
	}
}

func (s *StringLiteralContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitStringLiteral(s)
	}
}

type NumericLiteralContext struct {
	ConstantContext
}

func NewNumericLiteralContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *NumericLiteralContext {
	var p = new(NumericLiteralContext)

	InitEmptyConstantContext(&p.ConstantContext)
	p.parser = parser
	p.CopyAll(ctx.(*ConstantContext))

	return p
}

func (s *NumericLiteralContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *NumericLiteralContext) Number() INumberContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(INumberContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(INumberContext)
}

func (s *NumericLiteralContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterNumericLiteral(s)
	}
}

func (s *NumericLiteralContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitNumericLiteral(s)
	}
}

type BooleanLiteralContext struct {
	ConstantContext
}

func NewBooleanLiteralContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *BooleanLiteralContext {
	var p = new(BooleanLiteralContext)

	InitEmptyConstantContext(&p.ConstantContext)
	p.parser = parser
	p.CopyAll(ctx.(*ConstantContext))

	return p
}

func (s *BooleanLiteralContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BooleanLiteralContext) BooleanValue() IBooleanValueContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IBooleanValueContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IBooleanValueContext)
}

func (s *BooleanLiteralContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterBooleanLiteral(s)
	}
}

func (s *BooleanLiteralContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitBooleanLiteral(s)
	}
}

func (p *EqlBaseParser) Constant() (localctx IConstantContext) {
	localctx = NewConstantContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 46, EqlBaseParserRULE_constant)
	p.SetState(288)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case EqlBaseParserNULL:
		localctx = NewNullLiteralContext(p, localctx)
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(284)
			p.Match(EqlBaseParserNULL)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case EqlBaseParserINTEGER_VALUE, EqlBaseParserDECIMAL_VALUE:
		localctx = NewNumericLiteralContext(p, localctx)
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(285)
			p.Number()
		}

	case EqlBaseParserFALSE, EqlBaseParserTRUE:
		localctx = NewBooleanLiteralContext(p, localctx)
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(286)
			p.BooleanValue()
		}

	case EqlBaseParserSTRING:
		localctx = NewStringLiteralContext(p, localctx)
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(287)
			p.String_()
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IComparisonOperatorContext is an interface to support dynamic dispatch.
type IComparisonOperatorContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	EQ() antlr.TerminalNode
	NEQ() antlr.TerminalNode
	LT() antlr.TerminalNode
	LTE() antlr.TerminalNode
	GT() antlr.TerminalNode
	GTE() antlr.TerminalNode

	// IsComparisonOperatorContext differentiates from other interfaces.
	IsComparisonOperatorContext()
}

type ComparisonOperatorContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyComparisonOperatorContext() *ComparisonOperatorContext {
	var p = new(ComparisonOperatorContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_comparisonOperator
	return p
}

func InitEmptyComparisonOperatorContext(p *ComparisonOperatorContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_comparisonOperator
}

func (*ComparisonOperatorContext) IsComparisonOperatorContext() {}

func NewComparisonOperatorContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ComparisonOperatorContext {
	var p = new(ComparisonOperatorContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_comparisonOperator

	return p
}

func (s *ComparisonOperatorContext) GetParser() antlr.Parser { return s.parser }

func (s *ComparisonOperatorContext) EQ() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserEQ, 0)
}

func (s *ComparisonOperatorContext) NEQ() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserNEQ, 0)
}

func (s *ComparisonOperatorContext) LT() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserLT, 0)
}

func (s *ComparisonOperatorContext) LTE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserLTE, 0)
}

func (s *ComparisonOperatorContext) GT() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserGT, 0)
}

func (s *ComparisonOperatorContext) GTE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserGTE, 0)
}

func (s *ComparisonOperatorContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ComparisonOperatorContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ComparisonOperatorContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterComparisonOperator(s)
	}
}

func (s *ComparisonOperatorContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitComparisonOperator(s)
	}
}

func (p *EqlBaseParser) ComparisonOperator() (localctx IComparisonOperatorContext) {
	localctx = NewComparisonOperatorContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 48, EqlBaseParserRULE_comparisonOperator)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(290)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&2113929216) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IBooleanValueContext is an interface to support dynamic dispatch.
type IBooleanValueContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	TRUE() antlr.TerminalNode
	FALSE() antlr.TerminalNode

	// IsBooleanValueContext differentiates from other interfaces.
	IsBooleanValueContext()
}

type BooleanValueContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyBooleanValueContext() *BooleanValueContext {
	var p = new(BooleanValueContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_booleanValue
	return p
}

func InitEmptyBooleanValueContext(p *BooleanValueContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_booleanValue
}

func (*BooleanValueContext) IsBooleanValueContext() {}

func NewBooleanValueContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *BooleanValueContext {
	var p = new(BooleanValueContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_booleanValue

	return p
}

func (s *BooleanValueContext) GetParser() antlr.Parser { return s.parser }

func (s *BooleanValueContext) TRUE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserTRUE, 0)
}

func (s *BooleanValueContext) FALSE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserFALSE, 0)
}

func (s *BooleanValueContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BooleanValueContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *BooleanValueContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterBooleanValue(s)
	}
}

func (s *BooleanValueContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitBooleanValue(s)
	}
}

func (p *EqlBaseParser) BooleanValue() (localctx IBooleanValueContext) {
	localctx = NewBooleanValueContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 50, EqlBaseParserRULE_booleanValue)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(292)
		_la = p.GetTokenStream().LA(1)

		if !(_la == EqlBaseParserFALSE || _la == EqlBaseParserTRUE) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IQualifiedNameContext is an interface to support dynamic dispatch.
type IQualifiedNameContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	AllIdentifier() []IIdentifierContext
	Identifier(i int) IIdentifierContext
	OPTIONAL() antlr.TerminalNode
	AllDOT() []antlr.TerminalNode
	DOT(i int) antlr.TerminalNode
	AllLB() []antlr.TerminalNode
	LB(i int) antlr.TerminalNode
	AllRB() []antlr.TerminalNode
	RB(i int) antlr.TerminalNode
	AllINTEGER_VALUE() []antlr.TerminalNode
	INTEGER_VALUE(i int) antlr.TerminalNode

	// IsQualifiedNameContext differentiates from other interfaces.
	IsQualifiedNameContext()
}

type QualifiedNameContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyQualifiedNameContext() *QualifiedNameContext {
	var p = new(QualifiedNameContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_qualifiedName
	return p
}

func InitEmptyQualifiedNameContext(p *QualifiedNameContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_qualifiedName
}

func (*QualifiedNameContext) IsQualifiedNameContext() {}

func NewQualifiedNameContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *QualifiedNameContext {
	var p = new(QualifiedNameContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_qualifiedName

	return p
}

func (s *QualifiedNameContext) GetParser() antlr.Parser { return s.parser }

func (s *QualifiedNameContext) AllIdentifier() []IIdentifierContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IIdentifierContext); ok {
			len++
		}
	}

	tst := make([]IIdentifierContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IIdentifierContext); ok {
			tst[i] = t.(IIdentifierContext)
			i++
		}
	}

	return tst
}

func (s *QualifiedNameContext) Identifier(i int) IIdentifierContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IIdentifierContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IIdentifierContext)
}

func (s *QualifiedNameContext) OPTIONAL() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserOPTIONAL, 0)
}

func (s *QualifiedNameContext) AllDOT() []antlr.TerminalNode {
	return s.GetTokens(EqlBaseParserDOT)
}

func (s *QualifiedNameContext) DOT(i int) antlr.TerminalNode {
	return s.GetToken(EqlBaseParserDOT, i)
}

func (s *QualifiedNameContext) AllLB() []antlr.TerminalNode {
	return s.GetTokens(EqlBaseParserLB)
}

func (s *QualifiedNameContext) LB(i int) antlr.TerminalNode {
	return s.GetToken(EqlBaseParserLB, i)
}

func (s *QualifiedNameContext) AllRB() []antlr.TerminalNode {
	return s.GetTokens(EqlBaseParserRB)
}

func (s *QualifiedNameContext) RB(i int) antlr.TerminalNode {
	return s.GetToken(EqlBaseParserRB, i)
}

func (s *QualifiedNameContext) AllINTEGER_VALUE() []antlr.TerminalNode {
	return s.GetTokens(EqlBaseParserINTEGER_VALUE)
}

func (s *QualifiedNameContext) INTEGER_VALUE(i int) antlr.TerminalNode {
	return s.GetToken(EqlBaseParserINTEGER_VALUE, i)
}

func (s *QualifiedNameContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *QualifiedNameContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *QualifiedNameContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterQualifiedName(s)
	}
}

func (s *QualifiedNameContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitQualifiedName(s)
	}
}

func (p *EqlBaseParser) QualifiedName() (localctx IQualifiedNameContext) {
	localctx = NewQualifiedNameContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 52, EqlBaseParserRULE_qualifiedName)
	var _la int

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(295)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == EqlBaseParserOPTIONAL {
		{
			p.SetState(294)
			p.Match(EqlBaseParserOPTIONAL)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	}
	{
		p.SetState(297)
		p.Identifier()
	}
	p.SetState(309)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 38, p.GetParserRuleContext())
	if p.HasError() {
		goto errorExit
	}
	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			p.SetState(307)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}

			switch p.GetTokenStream().LA(1) {
			case EqlBaseParserDOT:
				{
					p.SetState(298)
					p.Match(EqlBaseParserDOT)
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				{
					p.SetState(299)
					p.Identifier()
				}

			case EqlBaseParserLB:
				{
					p.SetState(300)
					p.Match(EqlBaseParserLB)
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				p.SetState(302)
				p.GetErrorHandler().Sync(p)
				if p.HasError() {
					goto errorExit
				}
				_la = p.GetTokenStream().LA(1)

				for ok := true; ok; ok = _la == EqlBaseParserINTEGER_VALUE {
					{
						p.SetState(301)
						p.Match(EqlBaseParserINTEGER_VALUE)
						if p.HasError() {
							// Recognition error - abort rule
							goto errorExit
						}
					}

					p.SetState(304)
					p.GetErrorHandler().Sync(p)
					if p.HasError() {
						goto errorExit
					}
					_la = p.GetTokenStream().LA(1)
				}
				{
					p.SetState(306)
					p.Match(EqlBaseParserRB)
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}

			default:
				p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
				goto errorExit
			}

		}
		p.SetState(311)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 38, p.GetParserRuleContext())
		if p.HasError() {
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IIdentifierContext is an interface to support dynamic dispatch.
type IIdentifierContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	IDENTIFIER() antlr.TerminalNode
	QUOTED_IDENTIFIER() antlr.TerminalNode

	// IsIdentifierContext differentiates from other interfaces.
	IsIdentifierContext()
}

type IdentifierContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyIdentifierContext() *IdentifierContext {
	var p = new(IdentifierContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_identifier
	return p
}

func InitEmptyIdentifierContext(p *IdentifierContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_identifier
}

func (*IdentifierContext) IsIdentifierContext() {}

func NewIdentifierContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *IdentifierContext {
	var p = new(IdentifierContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_identifier

	return p
}

func (s *IdentifierContext) GetParser() antlr.Parser { return s.parser }

func (s *IdentifierContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserIDENTIFIER, 0)
}

func (s *IdentifierContext) QUOTED_IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserQUOTED_IDENTIFIER, 0)
}

func (s *IdentifierContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *IdentifierContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *IdentifierContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterIdentifier(s)
	}
}

func (s *IdentifierContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitIdentifier(s)
	}
}

func (p *EqlBaseParser) Identifier() (localctx IIdentifierContext) {
	localctx = NewIdentifierContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 54, EqlBaseParserRULE_identifier)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(312)
		_la = p.GetTokenStream().LA(1)

		if !(_la == EqlBaseParserIDENTIFIER || _la == EqlBaseParserQUOTED_IDENTIFIER) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// ITimeUnitContext is an interface to support dynamic dispatch.
type ITimeUnitContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetUnit returns the unit token.
	GetUnit() antlr.Token

	// SetUnit sets the unit token.
	SetUnit(antlr.Token)

	// Getter signatures
	Number() INumberContext
	IDENTIFIER() antlr.TerminalNode

	// IsTimeUnitContext differentiates from other interfaces.
	IsTimeUnitContext()
}

type TimeUnitContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	unit   antlr.Token
}

func NewEmptyTimeUnitContext() *TimeUnitContext {
	var p = new(TimeUnitContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_timeUnit
	return p
}

func InitEmptyTimeUnitContext(p *TimeUnitContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_timeUnit
}

func (*TimeUnitContext) IsTimeUnitContext() {}

func NewTimeUnitContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *TimeUnitContext {
	var p = new(TimeUnitContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_timeUnit

	return p
}

func (s *TimeUnitContext) GetParser() antlr.Parser { return s.parser }

func (s *TimeUnitContext) GetUnit() antlr.Token { return s.unit }

func (s *TimeUnitContext) SetUnit(v antlr.Token) { s.unit = v }

func (s *TimeUnitContext) Number() INumberContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(INumberContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(INumberContext)
}

func (s *TimeUnitContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserIDENTIFIER, 0)
}

func (s *TimeUnitContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *TimeUnitContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *TimeUnitContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterTimeUnit(s)
	}
}

func (s *TimeUnitContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitTimeUnit(s)
	}
}

func (p *EqlBaseParser) TimeUnit() (localctx ITimeUnitContext) {
	localctx = NewTimeUnitContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 56, EqlBaseParserRULE_timeUnit)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(314)
		p.Number()
	}
	p.SetState(316)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == EqlBaseParserIDENTIFIER {
		{
			p.SetState(315)

			var _m = p.Match(EqlBaseParserIDENTIFIER)

			localctx.(*TimeUnitContext).unit = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// INumberContext is an interface to support dynamic dispatch.
type INumberContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsNumberContext differentiates from other interfaces.
	IsNumberContext()
}

type NumberContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyNumberContext() *NumberContext {
	var p = new(NumberContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_number
	return p
}

func InitEmptyNumberContext(p *NumberContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_number
}

func (*NumberContext) IsNumberContext() {}

func NewNumberContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *NumberContext {
	var p = new(NumberContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_number

	return p
}

func (s *NumberContext) GetParser() antlr.Parser { return s.parser }

func (s *NumberContext) CopyAll(ctx *NumberContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *NumberContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *NumberContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type DecimalLiteralContext struct {
	NumberContext
}

func NewDecimalLiteralContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *DecimalLiteralContext {
	var p = new(DecimalLiteralContext)

	InitEmptyNumberContext(&p.NumberContext)
	p.parser = parser
	p.CopyAll(ctx.(*NumberContext))

	return p
}

func (s *DecimalLiteralContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DecimalLiteralContext) DECIMAL_VALUE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserDECIMAL_VALUE, 0)
}

func (s *DecimalLiteralContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterDecimalLiteral(s)
	}
}

func (s *DecimalLiteralContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitDecimalLiteral(s)
	}
}

type IntegerLiteralContext struct {
	NumberContext
}

func NewIntegerLiteralContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *IntegerLiteralContext {
	var p = new(IntegerLiteralContext)

	InitEmptyNumberContext(&p.NumberContext)
	p.parser = parser
	p.CopyAll(ctx.(*NumberContext))

	return p
}

func (s *IntegerLiteralContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *IntegerLiteralContext) INTEGER_VALUE() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserINTEGER_VALUE, 0)
}

func (s *IntegerLiteralContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterIntegerLiteral(s)
	}
}

func (s *IntegerLiteralContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitIntegerLiteral(s)
	}
}

func (p *EqlBaseParser) Number() (localctx INumberContext) {
	localctx = NewNumberContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 58, EqlBaseParserRULE_number)
	p.SetState(320)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case EqlBaseParserDECIMAL_VALUE:
		localctx = NewDecimalLiteralContext(p, localctx)
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(318)
			p.Match(EqlBaseParserDECIMAL_VALUE)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case EqlBaseParserINTEGER_VALUE:
		localctx = NewIntegerLiteralContext(p, localctx)
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(319)
			p.Match(EqlBaseParserINTEGER_VALUE)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IStringContext is an interface to support dynamic dispatch.
type IStringContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	STRING() antlr.TerminalNode

	// IsStringContext differentiates from other interfaces.
	IsStringContext()
}

type StringContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyStringContext() *StringContext {
	var p = new(StringContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_string
	return p
}

func InitEmptyStringContext(p *StringContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_string
}

func (*StringContext) IsStringContext() {}

func NewStringContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *StringContext {
	var p = new(StringContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_string

	return p
}

func (s *StringContext) GetParser() antlr.Parser { return s.parser }

func (s *StringContext) STRING() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserSTRING, 0)
}

func (s *StringContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *StringContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *StringContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterString(s)
	}
}

func (s *StringContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitString(s)
	}
}

func (p *EqlBaseParser) String_() (localctx IStringContext) {
	localctx = NewStringContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 60, EqlBaseParserRULE_string)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(322)
		p.Match(EqlBaseParserSTRING)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

// IEventValueContext is an interface to support dynamic dispatch.
type IEventValueContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	STRING() antlr.TerminalNode
	IDENTIFIER() antlr.TerminalNode

	// IsEventValueContext differentiates from other interfaces.
	IsEventValueContext()
}

type EventValueContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyEventValueContext() *EventValueContext {
	var p = new(EventValueContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_eventValue
	return p
}

func InitEmptyEventValueContext(p *EventValueContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EqlBaseParserRULE_eventValue
}

func (*EventValueContext) IsEventValueContext() {}

func NewEventValueContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *EventValueContext {
	var p = new(EventValueContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EqlBaseParserRULE_eventValue

	return p
}

func (s *EventValueContext) GetParser() antlr.Parser { return s.parser }

func (s *EventValueContext) STRING() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserSTRING, 0)
}

func (s *EventValueContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(EqlBaseParserIDENTIFIER, 0)
}

func (s *EventValueContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *EventValueContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *EventValueContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.EnterEventValue(s)
	}
}

func (s *EventValueContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EqlBaseListener); ok {
		listenerT.ExitEventValue(s)
	}
}

func (p *EqlBaseParser) EventValue() (localctx IEventValueContext) {
	localctx = NewEventValueContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 62, EqlBaseParserRULE_eventValue)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(324)
		_la = p.GetTokenStream().LA(1)

		if !(_la == EqlBaseParserSTRING || _la == EqlBaseParserIDENTIFIER) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
}

func (p *EqlBaseParser) Sempred(localctx antlr.RuleContext, ruleIndex, predIndex int) bool {
	switch ruleIndex {
	case 16:
		var t *BooleanExpressionContext = nil
		if localctx != nil {
			t = localctx.(*BooleanExpressionContext)
		}
		return p.BooleanExpression_Sempred(t, predIndex)

	case 18:
		var t *OperatorExpressionContext = nil
		if localctx != nil {
			t = localctx.(*OperatorExpressionContext)
		}
		return p.OperatorExpression_Sempred(t, predIndex)

	default:
		panic("No predicate with index: " + fmt.Sprint(ruleIndex))
	}
}

func (p *EqlBaseParser) BooleanExpression_Sempred(localctx antlr.RuleContext, predIndex int) bool {
	switch predIndex {
	case 0:
		return p.Precpred(p.GetParserRuleContext(), 2)

	case 1:
		return p.Precpred(p.GetParserRuleContext(), 1)

	default:
		panic("No predicate with index: " + fmt.Sprint(predIndex))
	}
}

func (p *EqlBaseParser) OperatorExpression_Sempred(localctx antlr.RuleContext, predIndex int) bool {
	switch predIndex {
	case 2:
		return p.Precpred(p.GetParserRuleContext(), 2)

	case 3:
		return p.Precpred(p.GetParserRuleContext(), 1)

	default:
		panic("No predicate with index: " + fmt.Sprint(predIndex))
	}
}
