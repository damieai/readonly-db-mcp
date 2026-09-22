// Code generated from tools/es-language-parser/upstream/sql9/SqlBase.g4 by ANTLR 4.13.1. DO NOT EDIT.

/*
 *  [2017] Elasticsearch Incorporated. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/** Fork of Facebook Presto Parser - significantly trimmed down and adjusted for ES */
/** presto-parser/src/main/antlr4/com/facebook/presto/sql/parser/SqlBase.g4 grammar */

package v9 // SqlBase
import "github.com/antlr4-go/antlr/v4"

// SqlBaseListener is a complete listener for a parse tree produced by SqlBaseParser.
type SqlBaseListener interface {
	antlr.ParseTreeListener

	// EnterSingleStatement is called when entering the singleStatement production.
	EnterSingleStatement(c *SingleStatementContext)

	// EnterSingleExpression is called when entering the singleExpression production.
	EnterSingleExpression(c *SingleExpressionContext)

	// EnterStatementDefault is called when entering the statementDefault production.
	EnterStatementDefault(c *StatementDefaultContext)

	// EnterExplain is called when entering the explain production.
	EnterExplain(c *ExplainContext)

	// EnterDebug is called when entering the debug production.
	EnterDebug(c *DebugContext)

	// EnterShowTables is called when entering the showTables production.
	EnterShowTables(c *ShowTablesContext)

	// EnterShowColumns is called when entering the showColumns production.
	EnterShowColumns(c *ShowColumnsContext)

	// EnterShowFunctions is called when entering the showFunctions production.
	EnterShowFunctions(c *ShowFunctionsContext)

	// EnterShowSchemas is called when entering the showSchemas production.
	EnterShowSchemas(c *ShowSchemasContext)

	// EnterShowCatalogs is called when entering the showCatalogs production.
	EnterShowCatalogs(c *ShowCatalogsContext)

	// EnterSysTables is called when entering the sysTables production.
	EnterSysTables(c *SysTablesContext)

	// EnterSysColumns is called when entering the sysColumns production.
	EnterSysColumns(c *SysColumnsContext)

	// EnterSysTypes is called when entering the sysTypes production.
	EnterSysTypes(c *SysTypesContext)

	// EnterQuery is called when entering the query production.
	EnterQuery(c *QueryContext)

	// EnterQueryNoWith is called when entering the queryNoWith production.
	EnterQueryNoWith(c *QueryNoWithContext)

	// EnterLimitClause is called when entering the limitClause production.
	EnterLimitClause(c *LimitClauseContext)

	// EnterQueryPrimaryDefault is called when entering the queryPrimaryDefault production.
	EnterQueryPrimaryDefault(c *QueryPrimaryDefaultContext)

	// EnterSubquery is called when entering the subquery production.
	EnterSubquery(c *SubqueryContext)

	// EnterOrderBy is called when entering the orderBy production.
	EnterOrderBy(c *OrderByContext)

	// EnterQuerySpecification is called when entering the querySpecification production.
	EnterQuerySpecification(c *QuerySpecificationContext)

	// EnterFromClause is called when entering the fromClause production.
	EnterFromClause(c *FromClauseContext)

	// EnterGroupBy is called when entering the groupBy production.
	EnterGroupBy(c *GroupByContext)

	// EnterSingleGroupingSet is called when entering the singleGroupingSet production.
	EnterSingleGroupingSet(c *SingleGroupingSetContext)

	// EnterGroupingExpressions is called when entering the groupingExpressions production.
	EnterGroupingExpressions(c *GroupingExpressionsContext)

	// EnterNamedQuery is called when entering the namedQuery production.
	EnterNamedQuery(c *NamedQueryContext)

	// EnterTopClause is called when entering the topClause production.
	EnterTopClause(c *TopClauseContext)

	// EnterSetQuantifier is called when entering the setQuantifier production.
	EnterSetQuantifier(c *SetQuantifierContext)

	// EnterSelectItems is called when entering the selectItems production.
	EnterSelectItems(c *SelectItemsContext)

	// EnterSelectExpression is called when entering the selectExpression production.
	EnterSelectExpression(c *SelectExpressionContext)

	// EnterRelation is called when entering the relation production.
	EnterRelation(c *RelationContext)

	// EnterJoinRelation is called when entering the joinRelation production.
	EnterJoinRelation(c *JoinRelationContext)

	// EnterJoinType is called when entering the joinType production.
	EnterJoinType(c *JoinTypeContext)

	// EnterJoinCriteria is called when entering the joinCriteria production.
	EnterJoinCriteria(c *JoinCriteriaContext)

	// EnterTableName is called when entering the tableName production.
	EnterTableName(c *TableNameContext)

	// EnterAliasedQuery is called when entering the aliasedQuery production.
	EnterAliasedQuery(c *AliasedQueryContext)

	// EnterAliasedRelation is called when entering the aliasedRelation production.
	EnterAliasedRelation(c *AliasedRelationContext)

	// EnterPivotClause is called when entering the pivotClause production.
	EnterPivotClause(c *PivotClauseContext)

	// EnterPivotArgs is called when entering the pivotArgs production.
	EnterPivotArgs(c *PivotArgsContext)

	// EnterNamedValueExpression is called when entering the namedValueExpression production.
	EnterNamedValueExpression(c *NamedValueExpressionContext)

	// EnterExpression is called when entering the expression production.
	EnterExpression(c *ExpressionContext)

	// EnterLogicalNot is called when entering the logicalNot production.
	EnterLogicalNot(c *LogicalNotContext)

	// EnterStringQuery is called when entering the stringQuery production.
	EnterStringQuery(c *StringQueryContext)

	// EnterBooleanDefault is called when entering the booleanDefault production.
	EnterBooleanDefault(c *BooleanDefaultContext)

	// EnterExists is called when entering the exists production.
	EnterExists(c *ExistsContext)

	// EnterMultiMatchQuery is called when entering the multiMatchQuery production.
	EnterMultiMatchQuery(c *MultiMatchQueryContext)

	// EnterMatchQuery is called when entering the matchQuery production.
	EnterMatchQuery(c *MatchQueryContext)

	// EnterLogicalBinary is called when entering the logicalBinary production.
	EnterLogicalBinary(c *LogicalBinaryContext)

	// EnterMatchQueryOptions is called when entering the matchQueryOptions production.
	EnterMatchQueryOptions(c *MatchQueryOptionsContext)

	// EnterPredicated is called when entering the predicated production.
	EnterPredicated(c *PredicatedContext)

	// EnterPredicate is called when entering the predicate production.
	EnterPredicate(c *PredicateContext)

	// EnterLikePattern is called when entering the likePattern production.
	EnterLikePattern(c *LikePatternContext)

	// EnterPattern is called when entering the pattern production.
	EnterPattern(c *PatternContext)

	// EnterPatternEscape is called when entering the patternEscape production.
	EnterPatternEscape(c *PatternEscapeContext)

	// EnterValueExpressionDefault is called when entering the valueExpressionDefault production.
	EnterValueExpressionDefault(c *ValueExpressionDefaultContext)

	// EnterComparison is called when entering the comparison production.
	EnterComparison(c *ComparisonContext)

	// EnterArithmeticBinary is called when entering the arithmeticBinary production.
	EnterArithmeticBinary(c *ArithmeticBinaryContext)

	// EnterArithmeticUnary is called when entering the arithmeticUnary production.
	EnterArithmeticUnary(c *ArithmeticUnaryContext)

	// EnterDereference is called when entering the dereference production.
	EnterDereference(c *DereferenceContext)

	// EnterCast is called when entering the cast production.
	EnterCast(c *CastContext)

	// EnterConstantDefault is called when entering the constantDefault production.
	EnterConstantDefault(c *ConstantDefaultContext)

	// EnterExtract is called when entering the extract production.
	EnterExtract(c *ExtractContext)

	// EnterParenthesizedExpression is called when entering the parenthesizedExpression production.
	EnterParenthesizedExpression(c *ParenthesizedExpressionContext)

	// EnterStar is called when entering the star production.
	EnterStar(c *StarContext)

	// EnterCastOperatorExpression is called when entering the castOperatorExpression production.
	EnterCastOperatorExpression(c *CastOperatorExpressionContext)

	// EnterFunction is called when entering the function production.
	EnterFunction(c *FunctionContext)

	// EnterCurrentDateTimeFunction is called when entering the currentDateTimeFunction production.
	EnterCurrentDateTimeFunction(c *CurrentDateTimeFunctionContext)

	// EnterSubqueryExpression is called when entering the subqueryExpression production.
	EnterSubqueryExpression(c *SubqueryExpressionContext)

	// EnterCase is called when entering the case production.
	EnterCase(c *CaseContext)

	// EnterBuiltinDateTimeFunction is called when entering the builtinDateTimeFunction production.
	EnterBuiltinDateTimeFunction(c *BuiltinDateTimeFunctionContext)

	// EnterCastExpression is called when entering the castExpression production.
	EnterCastExpression(c *CastExpressionContext)

	// EnterCastTemplate is called when entering the castTemplate production.
	EnterCastTemplate(c *CastTemplateContext)

	// EnterConvertTemplate is called when entering the convertTemplate production.
	EnterConvertTemplate(c *ConvertTemplateContext)

	// EnterExtractExpression is called when entering the extractExpression production.
	EnterExtractExpression(c *ExtractExpressionContext)

	// EnterExtractTemplate is called when entering the extractTemplate production.
	EnterExtractTemplate(c *ExtractTemplateContext)

	// EnterFunctionExpression is called when entering the functionExpression production.
	EnterFunctionExpression(c *FunctionExpressionContext)

	// EnterFunctionTemplate is called when entering the functionTemplate production.
	EnterFunctionTemplate(c *FunctionTemplateContext)

	// EnterFunctionName is called when entering the functionName production.
	EnterFunctionName(c *FunctionNameContext)

	// EnterNullLiteral is called when entering the nullLiteral production.
	EnterNullLiteral(c *NullLiteralContext)

	// EnterIntervalLiteral is called when entering the intervalLiteral production.
	EnterIntervalLiteral(c *IntervalLiteralContext)

	// EnterNumericLiteral is called when entering the numericLiteral production.
	EnterNumericLiteral(c *NumericLiteralContext)

	// EnterBooleanLiteral is called when entering the booleanLiteral production.
	EnterBooleanLiteral(c *BooleanLiteralContext)

	// EnterStringLiteral is called when entering the stringLiteral production.
	EnterStringLiteral(c *StringLiteralContext)

	// EnterParamLiteral is called when entering the paramLiteral production.
	EnterParamLiteral(c *ParamLiteralContext)

	// EnterDateEscapedLiteral is called when entering the dateEscapedLiteral production.
	EnterDateEscapedLiteral(c *DateEscapedLiteralContext)

	// EnterTimeEscapedLiteral is called when entering the timeEscapedLiteral production.
	EnterTimeEscapedLiteral(c *TimeEscapedLiteralContext)

	// EnterTimestampEscapedLiteral is called when entering the timestampEscapedLiteral production.
	EnterTimestampEscapedLiteral(c *TimestampEscapedLiteralContext)

	// EnterGuidEscapedLiteral is called when entering the guidEscapedLiteral production.
	EnterGuidEscapedLiteral(c *GuidEscapedLiteralContext)

	// EnterComparisonOperator is called when entering the comparisonOperator production.
	EnterComparisonOperator(c *ComparisonOperatorContext)

	// EnterBooleanValue is called when entering the booleanValue production.
	EnterBooleanValue(c *BooleanValueContext)

	// EnterInterval is called when entering the interval production.
	EnterInterval(c *IntervalContext)

	// EnterIntervalField is called when entering the intervalField production.
	EnterIntervalField(c *IntervalFieldContext)

	// EnterPrimitiveDataType is called when entering the primitiveDataType production.
	EnterPrimitiveDataType(c *PrimitiveDataTypeContext)

	// EnterQualifiedName is called when entering the qualifiedName production.
	EnterQualifiedName(c *QualifiedNameContext)

	// EnterIdentifier is called when entering the identifier production.
	EnterIdentifier(c *IdentifierContext)

	// EnterTableIdentifier is called when entering the tableIdentifier production.
	EnterTableIdentifier(c *TableIdentifierContext)

	// EnterQuotedIdentifier is called when entering the quotedIdentifier production.
	EnterQuotedIdentifier(c *QuotedIdentifierContext)

	// EnterBackQuotedIdentifier is called when entering the backQuotedIdentifier production.
	EnterBackQuotedIdentifier(c *BackQuotedIdentifierContext)

	// EnterUnquotedIdentifier is called when entering the unquotedIdentifier production.
	EnterUnquotedIdentifier(c *UnquotedIdentifierContext)

	// EnterDigitIdentifier is called when entering the digitIdentifier production.
	EnterDigitIdentifier(c *DigitIdentifierContext)

	// EnterDecimalLiteral is called when entering the decimalLiteral production.
	EnterDecimalLiteral(c *DecimalLiteralContext)

	// EnterIntegerLiteral is called when entering the integerLiteral production.
	EnterIntegerLiteral(c *IntegerLiteralContext)

	// EnterString is called when entering the string production.
	EnterString(c *StringContext)

	// EnterWhenClause is called when entering the whenClause production.
	EnterWhenClause(c *WhenClauseContext)

	// EnterNonReserved is called when entering the nonReserved production.
	EnterNonReserved(c *NonReservedContext)

	// ExitSingleStatement is called when exiting the singleStatement production.
	ExitSingleStatement(c *SingleStatementContext)

	// ExitSingleExpression is called when exiting the singleExpression production.
	ExitSingleExpression(c *SingleExpressionContext)

	// ExitStatementDefault is called when exiting the statementDefault production.
	ExitStatementDefault(c *StatementDefaultContext)

	// ExitExplain is called when exiting the explain production.
	ExitExplain(c *ExplainContext)

	// ExitDebug is called when exiting the debug production.
	ExitDebug(c *DebugContext)

	// ExitShowTables is called when exiting the showTables production.
	ExitShowTables(c *ShowTablesContext)

	// ExitShowColumns is called when exiting the showColumns production.
	ExitShowColumns(c *ShowColumnsContext)

	// ExitShowFunctions is called when exiting the showFunctions production.
	ExitShowFunctions(c *ShowFunctionsContext)

	// ExitShowSchemas is called when exiting the showSchemas production.
	ExitShowSchemas(c *ShowSchemasContext)

	// ExitShowCatalogs is called when exiting the showCatalogs production.
	ExitShowCatalogs(c *ShowCatalogsContext)

	// ExitSysTables is called when exiting the sysTables production.
	ExitSysTables(c *SysTablesContext)

	// ExitSysColumns is called when exiting the sysColumns production.
	ExitSysColumns(c *SysColumnsContext)

	// ExitSysTypes is called when exiting the sysTypes production.
	ExitSysTypes(c *SysTypesContext)

	// ExitQuery is called when exiting the query production.
	ExitQuery(c *QueryContext)

	// ExitQueryNoWith is called when exiting the queryNoWith production.
	ExitQueryNoWith(c *QueryNoWithContext)

	// ExitLimitClause is called when exiting the limitClause production.
	ExitLimitClause(c *LimitClauseContext)

	// ExitQueryPrimaryDefault is called when exiting the queryPrimaryDefault production.
	ExitQueryPrimaryDefault(c *QueryPrimaryDefaultContext)

	// ExitSubquery is called when exiting the subquery production.
	ExitSubquery(c *SubqueryContext)

	// ExitOrderBy is called when exiting the orderBy production.
	ExitOrderBy(c *OrderByContext)

	// ExitQuerySpecification is called when exiting the querySpecification production.
	ExitQuerySpecification(c *QuerySpecificationContext)

	// ExitFromClause is called when exiting the fromClause production.
	ExitFromClause(c *FromClauseContext)

	// ExitGroupBy is called when exiting the groupBy production.
	ExitGroupBy(c *GroupByContext)

	// ExitSingleGroupingSet is called when exiting the singleGroupingSet production.
	ExitSingleGroupingSet(c *SingleGroupingSetContext)

	// ExitGroupingExpressions is called when exiting the groupingExpressions production.
	ExitGroupingExpressions(c *GroupingExpressionsContext)

	// ExitNamedQuery is called when exiting the namedQuery production.
	ExitNamedQuery(c *NamedQueryContext)

	// ExitTopClause is called when exiting the topClause production.
	ExitTopClause(c *TopClauseContext)

	// ExitSetQuantifier is called when exiting the setQuantifier production.
	ExitSetQuantifier(c *SetQuantifierContext)

	// ExitSelectItems is called when exiting the selectItems production.
	ExitSelectItems(c *SelectItemsContext)

	// ExitSelectExpression is called when exiting the selectExpression production.
	ExitSelectExpression(c *SelectExpressionContext)

	// ExitRelation is called when exiting the relation production.
	ExitRelation(c *RelationContext)

	// ExitJoinRelation is called when exiting the joinRelation production.
	ExitJoinRelation(c *JoinRelationContext)

	// ExitJoinType is called when exiting the joinType production.
	ExitJoinType(c *JoinTypeContext)

	// ExitJoinCriteria is called when exiting the joinCriteria production.
	ExitJoinCriteria(c *JoinCriteriaContext)

	// ExitTableName is called when exiting the tableName production.
	ExitTableName(c *TableNameContext)

	// ExitAliasedQuery is called when exiting the aliasedQuery production.
	ExitAliasedQuery(c *AliasedQueryContext)

	// ExitAliasedRelation is called when exiting the aliasedRelation production.
	ExitAliasedRelation(c *AliasedRelationContext)

	// ExitPivotClause is called when exiting the pivotClause production.
	ExitPivotClause(c *PivotClauseContext)

	// ExitPivotArgs is called when exiting the pivotArgs production.
	ExitPivotArgs(c *PivotArgsContext)

	// ExitNamedValueExpression is called when exiting the namedValueExpression production.
	ExitNamedValueExpression(c *NamedValueExpressionContext)

	// ExitExpression is called when exiting the expression production.
	ExitExpression(c *ExpressionContext)

	// ExitLogicalNot is called when exiting the logicalNot production.
	ExitLogicalNot(c *LogicalNotContext)

	// ExitStringQuery is called when exiting the stringQuery production.
	ExitStringQuery(c *StringQueryContext)

	// ExitBooleanDefault is called when exiting the booleanDefault production.
	ExitBooleanDefault(c *BooleanDefaultContext)

	// ExitExists is called when exiting the exists production.
	ExitExists(c *ExistsContext)

	// ExitMultiMatchQuery is called when exiting the multiMatchQuery production.
	ExitMultiMatchQuery(c *MultiMatchQueryContext)

	// ExitMatchQuery is called when exiting the matchQuery production.
	ExitMatchQuery(c *MatchQueryContext)

	// ExitLogicalBinary is called when exiting the logicalBinary production.
	ExitLogicalBinary(c *LogicalBinaryContext)

	// ExitMatchQueryOptions is called when exiting the matchQueryOptions production.
	ExitMatchQueryOptions(c *MatchQueryOptionsContext)

	// ExitPredicated is called when exiting the predicated production.
	ExitPredicated(c *PredicatedContext)

	// ExitPredicate is called when exiting the predicate production.
	ExitPredicate(c *PredicateContext)

	// ExitLikePattern is called when exiting the likePattern production.
	ExitLikePattern(c *LikePatternContext)

	// ExitPattern is called when exiting the pattern production.
	ExitPattern(c *PatternContext)

	// ExitPatternEscape is called when exiting the patternEscape production.
	ExitPatternEscape(c *PatternEscapeContext)

	// ExitValueExpressionDefault is called when exiting the valueExpressionDefault production.
	ExitValueExpressionDefault(c *ValueExpressionDefaultContext)

	// ExitComparison is called when exiting the comparison production.
	ExitComparison(c *ComparisonContext)

	// ExitArithmeticBinary is called when exiting the arithmeticBinary production.
	ExitArithmeticBinary(c *ArithmeticBinaryContext)

	// ExitArithmeticUnary is called when exiting the arithmeticUnary production.
	ExitArithmeticUnary(c *ArithmeticUnaryContext)

	// ExitDereference is called when exiting the dereference production.
	ExitDereference(c *DereferenceContext)

	// ExitCast is called when exiting the cast production.
	ExitCast(c *CastContext)

	// ExitConstantDefault is called when exiting the constantDefault production.
	ExitConstantDefault(c *ConstantDefaultContext)

	// ExitExtract is called when exiting the extract production.
	ExitExtract(c *ExtractContext)

	// ExitParenthesizedExpression is called when exiting the parenthesizedExpression production.
	ExitParenthesizedExpression(c *ParenthesizedExpressionContext)

	// ExitStar is called when exiting the star production.
	ExitStar(c *StarContext)

	// ExitCastOperatorExpression is called when exiting the castOperatorExpression production.
	ExitCastOperatorExpression(c *CastOperatorExpressionContext)

	// ExitFunction is called when exiting the function production.
	ExitFunction(c *FunctionContext)

	// ExitCurrentDateTimeFunction is called when exiting the currentDateTimeFunction production.
	ExitCurrentDateTimeFunction(c *CurrentDateTimeFunctionContext)

	// ExitSubqueryExpression is called when exiting the subqueryExpression production.
	ExitSubqueryExpression(c *SubqueryExpressionContext)

	// ExitCase is called when exiting the case production.
	ExitCase(c *CaseContext)

	// ExitBuiltinDateTimeFunction is called when exiting the builtinDateTimeFunction production.
	ExitBuiltinDateTimeFunction(c *BuiltinDateTimeFunctionContext)

	// ExitCastExpression is called when exiting the castExpression production.
	ExitCastExpression(c *CastExpressionContext)

	// ExitCastTemplate is called when exiting the castTemplate production.
	ExitCastTemplate(c *CastTemplateContext)

	// ExitConvertTemplate is called when exiting the convertTemplate production.
	ExitConvertTemplate(c *ConvertTemplateContext)

	// ExitExtractExpression is called when exiting the extractExpression production.
	ExitExtractExpression(c *ExtractExpressionContext)

	// ExitExtractTemplate is called when exiting the extractTemplate production.
	ExitExtractTemplate(c *ExtractTemplateContext)

	// ExitFunctionExpression is called when exiting the functionExpression production.
	ExitFunctionExpression(c *FunctionExpressionContext)

	// ExitFunctionTemplate is called when exiting the functionTemplate production.
	ExitFunctionTemplate(c *FunctionTemplateContext)

	// ExitFunctionName is called when exiting the functionName production.
	ExitFunctionName(c *FunctionNameContext)

	// ExitNullLiteral is called when exiting the nullLiteral production.
	ExitNullLiteral(c *NullLiteralContext)

	// ExitIntervalLiteral is called when exiting the intervalLiteral production.
	ExitIntervalLiteral(c *IntervalLiteralContext)

	// ExitNumericLiteral is called when exiting the numericLiteral production.
	ExitNumericLiteral(c *NumericLiteralContext)

	// ExitBooleanLiteral is called when exiting the booleanLiteral production.
	ExitBooleanLiteral(c *BooleanLiteralContext)

	// ExitStringLiteral is called when exiting the stringLiteral production.
	ExitStringLiteral(c *StringLiteralContext)

	// ExitParamLiteral is called when exiting the paramLiteral production.
	ExitParamLiteral(c *ParamLiteralContext)

	// ExitDateEscapedLiteral is called when exiting the dateEscapedLiteral production.
	ExitDateEscapedLiteral(c *DateEscapedLiteralContext)

	// ExitTimeEscapedLiteral is called when exiting the timeEscapedLiteral production.
	ExitTimeEscapedLiteral(c *TimeEscapedLiteralContext)

	// ExitTimestampEscapedLiteral is called when exiting the timestampEscapedLiteral production.
	ExitTimestampEscapedLiteral(c *TimestampEscapedLiteralContext)

	// ExitGuidEscapedLiteral is called when exiting the guidEscapedLiteral production.
	ExitGuidEscapedLiteral(c *GuidEscapedLiteralContext)

	// ExitComparisonOperator is called when exiting the comparisonOperator production.
	ExitComparisonOperator(c *ComparisonOperatorContext)

	// ExitBooleanValue is called when exiting the booleanValue production.
	ExitBooleanValue(c *BooleanValueContext)

	// ExitInterval is called when exiting the interval production.
	ExitInterval(c *IntervalContext)

	// ExitIntervalField is called when exiting the intervalField production.
	ExitIntervalField(c *IntervalFieldContext)

	// ExitPrimitiveDataType is called when exiting the primitiveDataType production.
	ExitPrimitiveDataType(c *PrimitiveDataTypeContext)

	// ExitQualifiedName is called when exiting the qualifiedName production.
	ExitQualifiedName(c *QualifiedNameContext)

	// ExitIdentifier is called when exiting the identifier production.
	ExitIdentifier(c *IdentifierContext)

	// ExitTableIdentifier is called when exiting the tableIdentifier production.
	ExitTableIdentifier(c *TableIdentifierContext)

	// ExitQuotedIdentifier is called when exiting the quotedIdentifier production.
	ExitQuotedIdentifier(c *QuotedIdentifierContext)

	// ExitBackQuotedIdentifier is called when exiting the backQuotedIdentifier production.
	ExitBackQuotedIdentifier(c *BackQuotedIdentifierContext)

	// ExitUnquotedIdentifier is called when exiting the unquotedIdentifier production.
	ExitUnquotedIdentifier(c *UnquotedIdentifierContext)

	// ExitDigitIdentifier is called when exiting the digitIdentifier production.
	ExitDigitIdentifier(c *DigitIdentifierContext)

	// ExitDecimalLiteral is called when exiting the decimalLiteral production.
	ExitDecimalLiteral(c *DecimalLiteralContext)

	// ExitIntegerLiteral is called when exiting the integerLiteral production.
	ExitIntegerLiteral(c *IntegerLiteralContext)

	// ExitString is called when exiting the string production.
	ExitString(c *StringContext)

	// ExitWhenClause is called when exiting the whenClause production.
	ExitWhenClause(c *WhenClauseContext)

	// ExitNonReserved is called when exiting the nonReserved production.
	ExitNonReserved(c *NonReservedContext)
}
