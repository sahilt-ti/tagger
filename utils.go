package main

import (
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func inSlice[T comparable](elems []T, v T) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}

func getUncloseBracketsCount(bracketsCounters map[hclsyntax.TokenType]int) int {
	sum := 0
	for b := range bracketsCounters {
		sum += bracketsCounters[b]
	}

	return sum
}

func extractTagPairs(tokens hclwrite.Tokens) []hclwrite.Tokens {
	separatorTokens := []hclsyntax.TokenType{hclsyntax.TokenComma, hclsyntax.TokenNewline}

	bracketsCounters := map[hclsyntax.TokenType]int{
		hclsyntax.TokenOParen: 0,
		hclsyntax.TokenOBrack: 0,
	}

	openingBrackets := []hclsyntax.TokenType{hclsyntax.TokenOParen, hclsyntax.TokenOBrack}
	closingBrackets := []hclsyntax.TokenType{hclsyntax.TokenCParen, hclsyntax.TokenCBrack}

	bracketsPairs := map[hclsyntax.TokenType]hclsyntax.TokenType{
		hclsyntax.TokenCParen: hclsyntax.TokenOParen,
		hclsyntax.TokenCBrack: hclsyntax.TokenOBrack,
	}

	tagPairs := make([]hclwrite.Tokens, 0)
	startIndex := 0
	hasEq := false
	for i, token := range tokens {
		if inSlice(separatorTokens, token.Type) && getUncloseBracketsCount(bracketsCounters) == 0 {
			if hasEq {
				tagPairs = append(tagPairs, tokens[startIndex:i])
			}
			startIndex = i + 1
			hasEq = false
		}
		if token.Type == hclsyntax.TokenEqual {
			hasEq = true
		}
		if inSlice(openingBrackets, token.Type) {
			bracketsCounters[token.Type]++
		}
		if inSlice(closingBrackets, token.Type) {
			matchingOpen := bracketsPairs[token.Type]
			bracketsCounters[matchingOpen]--
		}
	}
	if hasEq {
		tagPairs = append(tagPairs, tokens[startIndex:])
	}

	return tagPairs
}

func getHclMapsContents(tokens hclwrite.Tokens) []hclwrite.Tokens {
	hclMaps := make([]hclwrite.Tokens, 0)
	bracketOpenIndex := -1

	for i, token := range tokens {
		if token.Type == hclsyntax.TokenOBrace {
			bracketOpenIndex = i
		}
		if token.Type == hclsyntax.TokenCBrace {
			hclMaps = append(hclMaps, tokens[bracketOpenIndex+1:i])
		}
	}

	return hclMaps
}

func parseTagAttribute(tokens hclwrite.Tokens) []hclwrite.ObjectAttrTokens {
	hclMaps := getHclMapsContents(tokens)
	tagPairs := make([]hclwrite.Tokens, 0)
	for _, hclMap := range hclMaps {
		tagPairs = append(tagPairs, extractTagPairs(hclMap)...)
	}

	tags := make([]hclwrite.ObjectAttrTokens, 0)
	for _, entry := range tagPairs {
		eqIndex := -1
		var key string
		for j, token := range entry {
			if token.Type == hclsyntax.TokenEqual {
				eqIndex = j + 1
				key = strings.TrimSpace(string(entry[:j].Bytes()))
			}
		}
		value := string(entry[eqIndex:].Bytes())
		value = strings.TrimPrefix(strings.TrimSuffix(value, " "), " ")
		
		tags = append(tags, hclwrite.ObjectAttrTokens{
			Name:  hclwrite.TokensForIdentifier(key),
			Value: hclwrite.TokensForIdentifier(value),
		})
	}
	return tags
}
