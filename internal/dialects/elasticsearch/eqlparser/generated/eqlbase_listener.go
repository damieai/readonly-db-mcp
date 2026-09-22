// Code generated from tools/es-language-parser/upstream/EqlBase.g4 by ANTLR 4.13.1. DO NOT EDIT.

/*
 * Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
 * or more contributor license agreements. Licensed under the Elastic License
 * 2.0; you may not use this file except in compliance with the Elastic License
 * 2.0.
 */

package generated // EqlBase
import "github.com/antlr4-go/antlr/v4"

// EqlBaseListener is a complete listener for a parse tree produced by EqlBaseParser.
type EqlBaseListener interface {
	antlr.ParseTreeListener

	// EnterSingleStatement is called when entering the singleStatement production.
	EnterSingleStatement(c *SingleStatementContext)

	// EnterSingleExpression is called when entering the singleExpression production.
	EnterSingleExpression(c *SingleExpressionContext)

	// EnterStatement is called when entering the statement production.
	EnterStatement(c *StatementContext)

	// EnterQuery is called when entering the query production.
	EnterQuery(c *QueryContext)

	// EnterSequenceParams is called when entering the sequenceParams production.
	EnterSequenceParams(c *SequenceParamsContext)

	// EnterSequence is called when entering the sequence production.
	EnterSequence(c *SequenceContext)

	// EnterSample is called when entering the sample production.
	EnterSample(c *SampleContext)

	// EnterJoin is called when entering the join production.
	EnterJoin(c *JoinContext)

	// EnterPipe is called when entering the pipe production.
	EnterPipe(c *PipeContext)

	// EnterJoinKeys is called when entering the joinKeys production.
	EnterJoinKeys(c *JoinKeysContext)

	// EnterJoinTerm is called when entering the joinTerm production.
	EnterJoinTerm(c *JoinTermContext)

	// EnterSequenceTerm is called when entering the sequenceTerm production.
	EnterSequenceTerm(c *SequenceTermContext)

	// EnterSubquery is called when entering the subquery production.
	EnterSubquery(c *SubqueryContext)

	// EnterEventQuery is called when entering the eventQuery production.
	EnterEventQuery(c *EventQueryContext)

	// EnterEventFilter is called when entering the eventFilter production.
	EnterEventFilter(c *EventFilterContext)

	// EnterExpression is called when entering the expression production.
	EnterExpression(c *ExpressionContext)

	// EnterLogicalNot is called when entering the logicalNot production.
	EnterLogicalNot(c *LogicalNotContext)

	// EnterBooleanDefault is called when entering the booleanDefault production.
	EnterBooleanDefault(c *BooleanDefaultContext)

	// EnterProcessCheck is called when entering the processCheck production.
	EnterProcessCheck(c *ProcessCheckContext)

	// EnterLogicalBinary is called when entering the logicalBinary production.
	EnterLogicalBinary(c *LogicalBinaryContext)

	// EnterValueExpressionDefault is called when entering the valueExpressionDefault production.
	EnterValueExpressionDefault(c *ValueExpressionDefaultContext)

	// EnterComparison is called when entering the comparison production.
	EnterComparison(c *ComparisonContext)

	// EnterOperatorExpressionDefault is called when entering the operatorExpressionDefault production.
	EnterOperatorExpressionDefault(c *OperatorExpressionDefaultContext)

	// EnterArithmeticBinary is called when entering the arithmeticBinary production.
	EnterArithmeticBinary(c *ArithmeticBinaryContext)

	// EnterArithmeticUnary is called when entering the arithmeticUnary production.
	EnterArithmeticUnary(c *ArithmeticUnaryContext)

	// EnterPredicate is called when entering the predicate production.
	EnterPredicate(c *PredicateContext)

	// EnterConstantDefault is called when entering the constantDefault production.
	EnterConstantDefault(c *ConstantDefaultContext)

	// EnterFunction is called when entering the function production.
	EnterFunction(c *FunctionContext)

	// EnterDereference is called when entering the dereference production.
	EnterDereference(c *DereferenceContext)

	// EnterParenthesizedExpression is called when entering the parenthesizedExpression production.
	EnterParenthesizedExpression(c *ParenthesizedExpressionContext)

	// EnterFunctionExpression is called when entering the functionExpression production.
	EnterFunctionExpression(c *FunctionExpressionContext)

	// EnterFunctionName is called when entering the functionName production.
	EnterFunctionName(c *FunctionNameContext)

	// EnterNullLiteral is called when entering the nullLiteral production.
	EnterNullLiteral(c *NullLiteralContext)

	// EnterNumericLiteral is called when entering the numericLiteral production.
	EnterNumericLiteral(c *NumericLiteralContext)

	// EnterBooleanLiteral is called when entering the booleanLiteral production.
	EnterBooleanLiteral(c *BooleanLiteralContext)

	// EnterStringLiteral is called when entering the stringLiteral production.
	EnterStringLiteral(c *StringLiteralContext)

	// EnterComparisonOperator is called when entering the comparisonOperator production.
	EnterComparisonOperator(c *ComparisonOperatorContext)

	// EnterBooleanValue is called when entering the booleanValue production.
	EnterBooleanValue(c *BooleanValueContext)

	// EnterQualifiedName is called when entering the qualifiedName production.
	EnterQualifiedName(c *QualifiedNameContext)

	// EnterIdentifier is called when entering the identifier production.
	EnterIdentifier(c *IdentifierContext)

	// EnterTimeUnit is called when entering the timeUnit production.
	EnterTimeUnit(c *TimeUnitContext)

	// EnterDecimalLiteral is called when entering the decimalLiteral production.
	EnterDecimalLiteral(c *DecimalLiteralContext)

	// EnterIntegerLiteral is called when entering the integerLiteral production.
	EnterIntegerLiteral(c *IntegerLiteralContext)

	// EnterString is called when entering the string production.
	EnterString(c *StringContext)

	// EnterEventValue is called when entering the eventValue production.
	EnterEventValue(c *EventValueContext)

	// ExitSingleStatement is called when exiting the singleStatement production.
	ExitSingleStatement(c *SingleStatementContext)

	// ExitSingleExpression is called when exiting the singleExpression production.
	ExitSingleExpression(c *SingleExpressionContext)

	// ExitStatement is called when exiting the statement production.
	ExitStatement(c *StatementContext)

	// ExitQuery is called when exiting the query production.
	ExitQuery(c *QueryContext)

	// ExitSequenceParams is called when exiting the sequenceParams production.
	ExitSequenceParams(c *SequenceParamsContext)

	// ExitSequence is called when exiting the sequence production.
	ExitSequence(c *SequenceContext)

	// ExitSample is called when exiting the sample production.
	ExitSample(c *SampleContext)

	// ExitJoin is called when exiting the join production.
	ExitJoin(c *JoinContext)

	// ExitPipe is called when exiting the pipe production.
	ExitPipe(c *PipeContext)

	// ExitJoinKeys is called when exiting the joinKeys production.
	ExitJoinKeys(c *JoinKeysContext)

	// ExitJoinTerm is called when exiting the joinTerm production.
	ExitJoinTerm(c *JoinTermContext)

	// ExitSequenceTerm is called when exiting the sequenceTerm production.
	ExitSequenceTerm(c *SequenceTermContext)

	// ExitSubquery is called when exiting the subquery production.
	ExitSubquery(c *SubqueryContext)

	// ExitEventQuery is called when exiting the eventQuery production.
	ExitEventQuery(c *EventQueryContext)

	// ExitEventFilter is called when exiting the eventFilter production.
	ExitEventFilter(c *EventFilterContext)

	// ExitExpression is called when exiting the expression production.
	ExitExpression(c *ExpressionContext)

	// ExitLogicalNot is called when exiting the logicalNot production.
	ExitLogicalNot(c *LogicalNotContext)

	// ExitBooleanDefault is called when exiting the booleanDefault production.
	ExitBooleanDefault(c *BooleanDefaultContext)

	// ExitProcessCheck is called when exiting the processCheck production.
	ExitProcessCheck(c *ProcessCheckContext)

	// ExitLogicalBinary is called when exiting the logicalBinary production.
	ExitLogicalBinary(c *LogicalBinaryContext)

	// ExitValueExpressionDefault is called when exiting the valueExpressionDefault production.
	ExitValueExpressionDefault(c *ValueExpressionDefaultContext)

	// ExitComparison is called when exiting the comparison production.
	ExitComparison(c *ComparisonContext)

	// ExitOperatorExpressionDefault is called when exiting the operatorExpressionDefault production.
	ExitOperatorExpressionDefault(c *OperatorExpressionDefaultContext)

	// ExitArithmeticBinary is called when exiting the arithmeticBinary production.
	ExitArithmeticBinary(c *ArithmeticBinaryContext)

	// ExitArithmeticUnary is called when exiting the arithmeticUnary production.
	ExitArithmeticUnary(c *ArithmeticUnaryContext)

	// ExitPredicate is called when exiting the predicate production.
	ExitPredicate(c *PredicateContext)

	// ExitConstantDefault is called when exiting the constantDefault production.
	ExitConstantDefault(c *ConstantDefaultContext)

	// ExitFunction is called when exiting the function production.
	ExitFunction(c *FunctionContext)

	// ExitDereference is called when exiting the dereference production.
	ExitDereference(c *DereferenceContext)

	// ExitParenthesizedExpression is called when exiting the parenthesizedExpression production.
	ExitParenthesizedExpression(c *ParenthesizedExpressionContext)

	// ExitFunctionExpression is called when exiting the functionExpression production.
	ExitFunctionExpression(c *FunctionExpressionContext)

	// ExitFunctionName is called when exiting the functionName production.
	ExitFunctionName(c *FunctionNameContext)

	// ExitNullLiteral is called when exiting the nullLiteral production.
	ExitNullLiteral(c *NullLiteralContext)

	// ExitNumericLiteral is called when exiting the numericLiteral production.
	ExitNumericLiteral(c *NumericLiteralContext)

	// ExitBooleanLiteral is called when exiting the booleanLiteral production.
	ExitBooleanLiteral(c *BooleanLiteralContext)

	// ExitStringLiteral is called when exiting the stringLiteral production.
	ExitStringLiteral(c *StringLiteralContext)

	// ExitComparisonOperator is called when exiting the comparisonOperator production.
	ExitComparisonOperator(c *ComparisonOperatorContext)

	// ExitBooleanValue is called when exiting the booleanValue production.
	ExitBooleanValue(c *BooleanValueContext)

	// ExitQualifiedName is called when exiting the qualifiedName production.
	ExitQualifiedName(c *QualifiedNameContext)

	// ExitIdentifier is called when exiting the identifier production.
	ExitIdentifier(c *IdentifierContext)

	// ExitTimeUnit is called when exiting the timeUnit production.
	ExitTimeUnit(c *TimeUnitContext)

	// ExitDecimalLiteral is called when exiting the decimalLiteral production.
	ExitDecimalLiteral(c *DecimalLiteralContext)

	// ExitIntegerLiteral is called when exiting the integerLiteral production.
	ExitIntegerLiteral(c *IntegerLiteralContext)

	// ExitString is called when exiting the string production.
	ExitString(c *StringContext)

	// ExitEventValue is called when exiting the eventValue production.
	ExitEventValue(c *EventValueContext)
}
