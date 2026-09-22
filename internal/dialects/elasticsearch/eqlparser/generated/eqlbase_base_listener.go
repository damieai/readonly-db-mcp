// Code generated from tools/es-language-parser/upstream/EqlBase.g4 by ANTLR 4.13.1. DO NOT EDIT.

/*
 * Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
 * or more contributor license agreements. Licensed under the Elastic License
 * 2.0; you may not use this file except in compliance with the Elastic License
 * 2.0.
 */

package generated // EqlBase
import "github.com/antlr4-go/antlr/v4"

// BaseEqlBaseListener is a complete listener for a parse tree produced by EqlBaseParser.
type BaseEqlBaseListener struct{}

var _ EqlBaseListener = &BaseEqlBaseListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseEqlBaseListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseEqlBaseListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseEqlBaseListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseEqlBaseListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterSingleStatement is called when production singleStatement is entered.
func (s *BaseEqlBaseListener) EnterSingleStatement(ctx *SingleStatementContext) {}

// ExitSingleStatement is called when production singleStatement is exited.
func (s *BaseEqlBaseListener) ExitSingleStatement(ctx *SingleStatementContext) {}

// EnterSingleExpression is called when production singleExpression is entered.
func (s *BaseEqlBaseListener) EnterSingleExpression(ctx *SingleExpressionContext) {}

// ExitSingleExpression is called when production singleExpression is exited.
func (s *BaseEqlBaseListener) ExitSingleExpression(ctx *SingleExpressionContext) {}

// EnterStatement is called when production statement is entered.
func (s *BaseEqlBaseListener) EnterStatement(ctx *StatementContext) {}

// ExitStatement is called when production statement is exited.
func (s *BaseEqlBaseListener) ExitStatement(ctx *StatementContext) {}

// EnterQuery is called when production query is entered.
func (s *BaseEqlBaseListener) EnterQuery(ctx *QueryContext) {}

// ExitQuery is called when production query is exited.
func (s *BaseEqlBaseListener) ExitQuery(ctx *QueryContext) {}

// EnterSequenceParams is called when production sequenceParams is entered.
func (s *BaseEqlBaseListener) EnterSequenceParams(ctx *SequenceParamsContext) {}

// ExitSequenceParams is called when production sequenceParams is exited.
func (s *BaseEqlBaseListener) ExitSequenceParams(ctx *SequenceParamsContext) {}

// EnterSequence is called when production sequence is entered.
func (s *BaseEqlBaseListener) EnterSequence(ctx *SequenceContext) {}

// ExitSequence is called when production sequence is exited.
func (s *BaseEqlBaseListener) ExitSequence(ctx *SequenceContext) {}

// EnterSample is called when production sample is entered.
func (s *BaseEqlBaseListener) EnterSample(ctx *SampleContext) {}

// ExitSample is called when production sample is exited.
func (s *BaseEqlBaseListener) ExitSample(ctx *SampleContext) {}

// EnterJoin is called when production join is entered.
func (s *BaseEqlBaseListener) EnterJoin(ctx *JoinContext) {}

// ExitJoin is called when production join is exited.
func (s *BaseEqlBaseListener) ExitJoin(ctx *JoinContext) {}

// EnterPipe is called when production pipe is entered.
func (s *BaseEqlBaseListener) EnterPipe(ctx *PipeContext) {}

// ExitPipe is called when production pipe is exited.
func (s *BaseEqlBaseListener) ExitPipe(ctx *PipeContext) {}

// EnterJoinKeys is called when production joinKeys is entered.
func (s *BaseEqlBaseListener) EnterJoinKeys(ctx *JoinKeysContext) {}

// ExitJoinKeys is called when production joinKeys is exited.
func (s *BaseEqlBaseListener) ExitJoinKeys(ctx *JoinKeysContext) {}

// EnterJoinTerm is called when production joinTerm is entered.
func (s *BaseEqlBaseListener) EnterJoinTerm(ctx *JoinTermContext) {}

// ExitJoinTerm is called when production joinTerm is exited.
func (s *BaseEqlBaseListener) ExitJoinTerm(ctx *JoinTermContext) {}

// EnterSequenceTerm is called when production sequenceTerm is entered.
func (s *BaseEqlBaseListener) EnterSequenceTerm(ctx *SequenceTermContext) {}

// ExitSequenceTerm is called when production sequenceTerm is exited.
func (s *BaseEqlBaseListener) ExitSequenceTerm(ctx *SequenceTermContext) {}

// EnterSubquery is called when production subquery is entered.
func (s *BaseEqlBaseListener) EnterSubquery(ctx *SubqueryContext) {}

// ExitSubquery is called when production subquery is exited.
func (s *BaseEqlBaseListener) ExitSubquery(ctx *SubqueryContext) {}

// EnterEventQuery is called when production eventQuery is entered.
func (s *BaseEqlBaseListener) EnterEventQuery(ctx *EventQueryContext) {}

// ExitEventQuery is called when production eventQuery is exited.
func (s *BaseEqlBaseListener) ExitEventQuery(ctx *EventQueryContext) {}

// EnterEventFilter is called when production eventFilter is entered.
func (s *BaseEqlBaseListener) EnterEventFilter(ctx *EventFilterContext) {}

// ExitEventFilter is called when production eventFilter is exited.
func (s *BaseEqlBaseListener) ExitEventFilter(ctx *EventFilterContext) {}

// EnterExpression is called when production expression is entered.
func (s *BaseEqlBaseListener) EnterExpression(ctx *ExpressionContext) {}

// ExitExpression is called when production expression is exited.
func (s *BaseEqlBaseListener) ExitExpression(ctx *ExpressionContext) {}

// EnterLogicalNot is called when production logicalNot is entered.
func (s *BaseEqlBaseListener) EnterLogicalNot(ctx *LogicalNotContext) {}

// ExitLogicalNot is called when production logicalNot is exited.
func (s *BaseEqlBaseListener) ExitLogicalNot(ctx *LogicalNotContext) {}

// EnterBooleanDefault is called when production booleanDefault is entered.
func (s *BaseEqlBaseListener) EnterBooleanDefault(ctx *BooleanDefaultContext) {}

// ExitBooleanDefault is called when production booleanDefault is exited.
func (s *BaseEqlBaseListener) ExitBooleanDefault(ctx *BooleanDefaultContext) {}

// EnterProcessCheck is called when production processCheck is entered.
func (s *BaseEqlBaseListener) EnterProcessCheck(ctx *ProcessCheckContext) {}

// ExitProcessCheck is called when production processCheck is exited.
func (s *BaseEqlBaseListener) ExitProcessCheck(ctx *ProcessCheckContext) {}

// EnterLogicalBinary is called when production logicalBinary is entered.
func (s *BaseEqlBaseListener) EnterLogicalBinary(ctx *LogicalBinaryContext) {}

// ExitLogicalBinary is called when production logicalBinary is exited.
func (s *BaseEqlBaseListener) ExitLogicalBinary(ctx *LogicalBinaryContext) {}

// EnterValueExpressionDefault is called when production valueExpressionDefault is entered.
func (s *BaseEqlBaseListener) EnterValueExpressionDefault(ctx *ValueExpressionDefaultContext) {}

// ExitValueExpressionDefault is called when production valueExpressionDefault is exited.
func (s *BaseEqlBaseListener) ExitValueExpressionDefault(ctx *ValueExpressionDefaultContext) {}

// EnterComparison is called when production comparison is entered.
func (s *BaseEqlBaseListener) EnterComparison(ctx *ComparisonContext) {}

// ExitComparison is called when production comparison is exited.
func (s *BaseEqlBaseListener) ExitComparison(ctx *ComparisonContext) {}

// EnterOperatorExpressionDefault is called when production operatorExpressionDefault is entered.
func (s *BaseEqlBaseListener) EnterOperatorExpressionDefault(ctx *OperatorExpressionDefaultContext) {}

// ExitOperatorExpressionDefault is called when production operatorExpressionDefault is exited.
func (s *BaseEqlBaseListener) ExitOperatorExpressionDefault(ctx *OperatorExpressionDefaultContext) {}

// EnterArithmeticBinary is called when production arithmeticBinary is entered.
func (s *BaseEqlBaseListener) EnterArithmeticBinary(ctx *ArithmeticBinaryContext) {}

// ExitArithmeticBinary is called when production arithmeticBinary is exited.
func (s *BaseEqlBaseListener) ExitArithmeticBinary(ctx *ArithmeticBinaryContext) {}

// EnterArithmeticUnary is called when production arithmeticUnary is entered.
func (s *BaseEqlBaseListener) EnterArithmeticUnary(ctx *ArithmeticUnaryContext) {}

// ExitArithmeticUnary is called when production arithmeticUnary is exited.
func (s *BaseEqlBaseListener) ExitArithmeticUnary(ctx *ArithmeticUnaryContext) {}

// EnterPredicate is called when production predicate is entered.
func (s *BaseEqlBaseListener) EnterPredicate(ctx *PredicateContext) {}

// ExitPredicate is called when production predicate is exited.
func (s *BaseEqlBaseListener) ExitPredicate(ctx *PredicateContext) {}

// EnterConstantDefault is called when production constantDefault is entered.
func (s *BaseEqlBaseListener) EnterConstantDefault(ctx *ConstantDefaultContext) {}

// ExitConstantDefault is called when production constantDefault is exited.
func (s *BaseEqlBaseListener) ExitConstantDefault(ctx *ConstantDefaultContext) {}

// EnterFunction is called when production function is entered.
func (s *BaseEqlBaseListener) EnterFunction(ctx *FunctionContext) {}

// ExitFunction is called when production function is exited.
func (s *BaseEqlBaseListener) ExitFunction(ctx *FunctionContext) {}

// EnterDereference is called when production dereference is entered.
func (s *BaseEqlBaseListener) EnterDereference(ctx *DereferenceContext) {}

// ExitDereference is called when production dereference is exited.
func (s *BaseEqlBaseListener) ExitDereference(ctx *DereferenceContext) {}

// EnterParenthesizedExpression is called when production parenthesizedExpression is entered.
func (s *BaseEqlBaseListener) EnterParenthesizedExpression(ctx *ParenthesizedExpressionContext) {}

// ExitParenthesizedExpression is called when production parenthesizedExpression is exited.
func (s *BaseEqlBaseListener) ExitParenthesizedExpression(ctx *ParenthesizedExpressionContext) {}

// EnterFunctionExpression is called when production functionExpression is entered.
func (s *BaseEqlBaseListener) EnterFunctionExpression(ctx *FunctionExpressionContext) {}

// ExitFunctionExpression is called when production functionExpression is exited.
func (s *BaseEqlBaseListener) ExitFunctionExpression(ctx *FunctionExpressionContext) {}

// EnterFunctionName is called when production functionName is entered.
func (s *BaseEqlBaseListener) EnterFunctionName(ctx *FunctionNameContext) {}

// ExitFunctionName is called when production functionName is exited.
func (s *BaseEqlBaseListener) ExitFunctionName(ctx *FunctionNameContext) {}

// EnterNullLiteral is called when production nullLiteral is entered.
func (s *BaseEqlBaseListener) EnterNullLiteral(ctx *NullLiteralContext) {}

// ExitNullLiteral is called when production nullLiteral is exited.
func (s *BaseEqlBaseListener) ExitNullLiteral(ctx *NullLiteralContext) {}

// EnterNumericLiteral is called when production numericLiteral is entered.
func (s *BaseEqlBaseListener) EnterNumericLiteral(ctx *NumericLiteralContext) {}

// ExitNumericLiteral is called when production numericLiteral is exited.
func (s *BaseEqlBaseListener) ExitNumericLiteral(ctx *NumericLiteralContext) {}

// EnterBooleanLiteral is called when production booleanLiteral is entered.
func (s *BaseEqlBaseListener) EnterBooleanLiteral(ctx *BooleanLiteralContext) {}

// ExitBooleanLiteral is called when production booleanLiteral is exited.
func (s *BaseEqlBaseListener) ExitBooleanLiteral(ctx *BooleanLiteralContext) {}

// EnterStringLiteral is called when production stringLiteral is entered.
func (s *BaseEqlBaseListener) EnterStringLiteral(ctx *StringLiteralContext) {}

// ExitStringLiteral is called when production stringLiteral is exited.
func (s *BaseEqlBaseListener) ExitStringLiteral(ctx *StringLiteralContext) {}

// EnterComparisonOperator is called when production comparisonOperator is entered.
func (s *BaseEqlBaseListener) EnterComparisonOperator(ctx *ComparisonOperatorContext) {}

// ExitComparisonOperator is called when production comparisonOperator is exited.
func (s *BaseEqlBaseListener) ExitComparisonOperator(ctx *ComparisonOperatorContext) {}

// EnterBooleanValue is called when production booleanValue is entered.
func (s *BaseEqlBaseListener) EnterBooleanValue(ctx *BooleanValueContext) {}

// ExitBooleanValue is called when production booleanValue is exited.
func (s *BaseEqlBaseListener) ExitBooleanValue(ctx *BooleanValueContext) {}

// EnterQualifiedName is called when production qualifiedName is entered.
func (s *BaseEqlBaseListener) EnterQualifiedName(ctx *QualifiedNameContext) {}

// ExitQualifiedName is called when production qualifiedName is exited.
func (s *BaseEqlBaseListener) ExitQualifiedName(ctx *QualifiedNameContext) {}

// EnterIdentifier is called when production identifier is entered.
func (s *BaseEqlBaseListener) EnterIdentifier(ctx *IdentifierContext) {}

// ExitIdentifier is called when production identifier is exited.
func (s *BaseEqlBaseListener) ExitIdentifier(ctx *IdentifierContext) {}

// EnterTimeUnit is called when production timeUnit is entered.
func (s *BaseEqlBaseListener) EnterTimeUnit(ctx *TimeUnitContext) {}

// ExitTimeUnit is called when production timeUnit is exited.
func (s *BaseEqlBaseListener) ExitTimeUnit(ctx *TimeUnitContext) {}

// EnterDecimalLiteral is called when production decimalLiteral is entered.
func (s *BaseEqlBaseListener) EnterDecimalLiteral(ctx *DecimalLiteralContext) {}

// ExitDecimalLiteral is called when production decimalLiteral is exited.
func (s *BaseEqlBaseListener) ExitDecimalLiteral(ctx *DecimalLiteralContext) {}

// EnterIntegerLiteral is called when production integerLiteral is entered.
func (s *BaseEqlBaseListener) EnterIntegerLiteral(ctx *IntegerLiteralContext) {}

// ExitIntegerLiteral is called when production integerLiteral is exited.
func (s *BaseEqlBaseListener) ExitIntegerLiteral(ctx *IntegerLiteralContext) {}

// EnterString is called when production string is entered.
func (s *BaseEqlBaseListener) EnterString(ctx *StringContext) {}

// ExitString is called when production string is exited.
func (s *BaseEqlBaseListener) ExitString(ctx *StringContext) {}

// EnterEventValue is called when production eventValue is entered.
func (s *BaseEqlBaseListener) EnterEventValue(ctx *EventValueContext) {}

// ExitEventValue is called when production eventValue is exited.
func (s *BaseEqlBaseListener) ExitEventValue(ctx *EventValueContext) {}
