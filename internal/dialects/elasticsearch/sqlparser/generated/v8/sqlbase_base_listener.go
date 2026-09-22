// Code generated from tools/es-language-parser/upstream/sql8/SqlBase.g4 by ANTLR 4.13.1. DO NOT EDIT.

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

package v8 // SqlBase
import "github.com/antlr4-go/antlr/v4"

// BaseSqlBaseListener is a complete listener for a parse tree produced by SqlBaseParser.
type BaseSqlBaseListener struct{}

var _ SqlBaseListener = &BaseSqlBaseListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseSqlBaseListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseSqlBaseListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseSqlBaseListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseSqlBaseListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterSingleStatement is called when production singleStatement is entered.
func (s *BaseSqlBaseListener) EnterSingleStatement(ctx *SingleStatementContext) {}

// ExitSingleStatement is called when production singleStatement is exited.
func (s *BaseSqlBaseListener) ExitSingleStatement(ctx *SingleStatementContext) {}

// EnterSingleExpression is called when production singleExpression is entered.
func (s *BaseSqlBaseListener) EnterSingleExpression(ctx *SingleExpressionContext) {}

// ExitSingleExpression is called when production singleExpression is exited.
func (s *BaseSqlBaseListener) ExitSingleExpression(ctx *SingleExpressionContext) {}

// EnterStatementDefault is called when production statementDefault is entered.
func (s *BaseSqlBaseListener) EnterStatementDefault(ctx *StatementDefaultContext) {}

// ExitStatementDefault is called when production statementDefault is exited.
func (s *BaseSqlBaseListener) ExitStatementDefault(ctx *StatementDefaultContext) {}

// EnterExplain is called when production explain is entered.
func (s *BaseSqlBaseListener) EnterExplain(ctx *ExplainContext) {}

// ExitExplain is called when production explain is exited.
func (s *BaseSqlBaseListener) ExitExplain(ctx *ExplainContext) {}

// EnterDebug is called when production debug is entered.
func (s *BaseSqlBaseListener) EnterDebug(ctx *DebugContext) {}

// ExitDebug is called when production debug is exited.
func (s *BaseSqlBaseListener) ExitDebug(ctx *DebugContext) {}

// EnterShowTables is called when production showTables is entered.
func (s *BaseSqlBaseListener) EnterShowTables(ctx *ShowTablesContext) {}

// ExitShowTables is called when production showTables is exited.
func (s *BaseSqlBaseListener) ExitShowTables(ctx *ShowTablesContext) {}

// EnterShowColumns is called when production showColumns is entered.
func (s *BaseSqlBaseListener) EnterShowColumns(ctx *ShowColumnsContext) {}

// ExitShowColumns is called when production showColumns is exited.
func (s *BaseSqlBaseListener) ExitShowColumns(ctx *ShowColumnsContext) {}

// EnterShowFunctions is called when production showFunctions is entered.
func (s *BaseSqlBaseListener) EnterShowFunctions(ctx *ShowFunctionsContext) {}

// ExitShowFunctions is called when production showFunctions is exited.
func (s *BaseSqlBaseListener) ExitShowFunctions(ctx *ShowFunctionsContext) {}

// EnterShowSchemas is called when production showSchemas is entered.
func (s *BaseSqlBaseListener) EnterShowSchemas(ctx *ShowSchemasContext) {}

// ExitShowSchemas is called when production showSchemas is exited.
func (s *BaseSqlBaseListener) ExitShowSchemas(ctx *ShowSchemasContext) {}

// EnterShowCatalogs is called when production showCatalogs is entered.
func (s *BaseSqlBaseListener) EnterShowCatalogs(ctx *ShowCatalogsContext) {}

// ExitShowCatalogs is called when production showCatalogs is exited.
func (s *BaseSqlBaseListener) ExitShowCatalogs(ctx *ShowCatalogsContext) {}

// EnterSysTables is called when production sysTables is entered.
func (s *BaseSqlBaseListener) EnterSysTables(ctx *SysTablesContext) {}

// ExitSysTables is called when production sysTables is exited.
func (s *BaseSqlBaseListener) ExitSysTables(ctx *SysTablesContext) {}

// EnterSysColumns is called when production sysColumns is entered.
func (s *BaseSqlBaseListener) EnterSysColumns(ctx *SysColumnsContext) {}

// ExitSysColumns is called when production sysColumns is exited.
func (s *BaseSqlBaseListener) ExitSysColumns(ctx *SysColumnsContext) {}

// EnterSysTypes is called when production sysTypes is entered.
func (s *BaseSqlBaseListener) EnterSysTypes(ctx *SysTypesContext) {}

// ExitSysTypes is called when production sysTypes is exited.
func (s *BaseSqlBaseListener) ExitSysTypes(ctx *SysTypesContext) {}

// EnterQuery is called when production query is entered.
func (s *BaseSqlBaseListener) EnterQuery(ctx *QueryContext) {}

// ExitQuery is called when production query is exited.
func (s *BaseSqlBaseListener) ExitQuery(ctx *QueryContext) {}

// EnterQueryNoWith is called when production queryNoWith is entered.
func (s *BaseSqlBaseListener) EnterQueryNoWith(ctx *QueryNoWithContext) {}

// ExitQueryNoWith is called when production queryNoWith is exited.
func (s *BaseSqlBaseListener) ExitQueryNoWith(ctx *QueryNoWithContext) {}

// EnterLimitClause is called when production limitClause is entered.
func (s *BaseSqlBaseListener) EnterLimitClause(ctx *LimitClauseContext) {}

// ExitLimitClause is called when production limitClause is exited.
func (s *BaseSqlBaseListener) ExitLimitClause(ctx *LimitClauseContext) {}

// EnterQueryPrimaryDefault is called when production queryPrimaryDefault is entered.
func (s *BaseSqlBaseListener) EnterQueryPrimaryDefault(ctx *QueryPrimaryDefaultContext) {}

// ExitQueryPrimaryDefault is called when production queryPrimaryDefault is exited.
func (s *BaseSqlBaseListener) ExitQueryPrimaryDefault(ctx *QueryPrimaryDefaultContext) {}

// EnterSubquery is called when production subquery is entered.
func (s *BaseSqlBaseListener) EnterSubquery(ctx *SubqueryContext) {}

// ExitSubquery is called when production subquery is exited.
func (s *BaseSqlBaseListener) ExitSubquery(ctx *SubqueryContext) {}

// EnterOrderBy is called when production orderBy is entered.
func (s *BaseSqlBaseListener) EnterOrderBy(ctx *OrderByContext) {}

// ExitOrderBy is called when production orderBy is exited.
func (s *BaseSqlBaseListener) ExitOrderBy(ctx *OrderByContext) {}

// EnterQuerySpecification is called when production querySpecification is entered.
func (s *BaseSqlBaseListener) EnterQuerySpecification(ctx *QuerySpecificationContext) {}

// ExitQuerySpecification is called when production querySpecification is exited.
func (s *BaseSqlBaseListener) ExitQuerySpecification(ctx *QuerySpecificationContext) {}

// EnterFromClause is called when production fromClause is entered.
func (s *BaseSqlBaseListener) EnterFromClause(ctx *FromClauseContext) {}

// ExitFromClause is called when production fromClause is exited.
func (s *BaseSqlBaseListener) ExitFromClause(ctx *FromClauseContext) {}

// EnterGroupBy is called when production groupBy is entered.
func (s *BaseSqlBaseListener) EnterGroupBy(ctx *GroupByContext) {}

// ExitGroupBy is called when production groupBy is exited.
func (s *BaseSqlBaseListener) ExitGroupBy(ctx *GroupByContext) {}

// EnterSingleGroupingSet is called when production singleGroupingSet is entered.
func (s *BaseSqlBaseListener) EnterSingleGroupingSet(ctx *SingleGroupingSetContext) {}

// ExitSingleGroupingSet is called when production singleGroupingSet is exited.
func (s *BaseSqlBaseListener) ExitSingleGroupingSet(ctx *SingleGroupingSetContext) {}

// EnterGroupingExpressions is called when production groupingExpressions is entered.
func (s *BaseSqlBaseListener) EnterGroupingExpressions(ctx *GroupingExpressionsContext) {}

// ExitGroupingExpressions is called when production groupingExpressions is exited.
func (s *BaseSqlBaseListener) ExitGroupingExpressions(ctx *GroupingExpressionsContext) {}

// EnterNamedQuery is called when production namedQuery is entered.
func (s *BaseSqlBaseListener) EnterNamedQuery(ctx *NamedQueryContext) {}

// ExitNamedQuery is called when production namedQuery is exited.
func (s *BaseSqlBaseListener) ExitNamedQuery(ctx *NamedQueryContext) {}

// EnterTopClause is called when production topClause is entered.
func (s *BaseSqlBaseListener) EnterTopClause(ctx *TopClauseContext) {}

// ExitTopClause is called when production topClause is exited.
func (s *BaseSqlBaseListener) ExitTopClause(ctx *TopClauseContext) {}

// EnterSetQuantifier is called when production setQuantifier is entered.
func (s *BaseSqlBaseListener) EnterSetQuantifier(ctx *SetQuantifierContext) {}

// ExitSetQuantifier is called when production setQuantifier is exited.
func (s *BaseSqlBaseListener) ExitSetQuantifier(ctx *SetQuantifierContext) {}

// EnterSelectItems is called when production selectItems is entered.
func (s *BaseSqlBaseListener) EnterSelectItems(ctx *SelectItemsContext) {}

// ExitSelectItems is called when production selectItems is exited.
func (s *BaseSqlBaseListener) ExitSelectItems(ctx *SelectItemsContext) {}

// EnterSelectExpression is called when production selectExpression is entered.
func (s *BaseSqlBaseListener) EnterSelectExpression(ctx *SelectExpressionContext) {}

// ExitSelectExpression is called when production selectExpression is exited.
func (s *BaseSqlBaseListener) ExitSelectExpression(ctx *SelectExpressionContext) {}

// EnterRelation is called when production relation is entered.
func (s *BaseSqlBaseListener) EnterRelation(ctx *RelationContext) {}

// ExitRelation is called when production relation is exited.
func (s *BaseSqlBaseListener) ExitRelation(ctx *RelationContext) {}

// EnterJoinRelation is called when production joinRelation is entered.
func (s *BaseSqlBaseListener) EnterJoinRelation(ctx *JoinRelationContext) {}

// ExitJoinRelation is called when production joinRelation is exited.
func (s *BaseSqlBaseListener) ExitJoinRelation(ctx *JoinRelationContext) {}

// EnterJoinType is called when production joinType is entered.
func (s *BaseSqlBaseListener) EnterJoinType(ctx *JoinTypeContext) {}

// ExitJoinType is called when production joinType is exited.
func (s *BaseSqlBaseListener) ExitJoinType(ctx *JoinTypeContext) {}

// EnterJoinCriteria is called when production joinCriteria is entered.
func (s *BaseSqlBaseListener) EnterJoinCriteria(ctx *JoinCriteriaContext) {}

// ExitJoinCriteria is called when production joinCriteria is exited.
func (s *BaseSqlBaseListener) ExitJoinCriteria(ctx *JoinCriteriaContext) {}

// EnterTableName is called when production tableName is entered.
func (s *BaseSqlBaseListener) EnterTableName(ctx *TableNameContext) {}

// ExitTableName is called when production tableName is exited.
func (s *BaseSqlBaseListener) ExitTableName(ctx *TableNameContext) {}

// EnterAliasedQuery is called when production aliasedQuery is entered.
func (s *BaseSqlBaseListener) EnterAliasedQuery(ctx *AliasedQueryContext) {}

// ExitAliasedQuery is called when production aliasedQuery is exited.
func (s *BaseSqlBaseListener) ExitAliasedQuery(ctx *AliasedQueryContext) {}

// EnterAliasedRelation is called when production aliasedRelation is entered.
func (s *BaseSqlBaseListener) EnterAliasedRelation(ctx *AliasedRelationContext) {}

// ExitAliasedRelation is called when production aliasedRelation is exited.
func (s *BaseSqlBaseListener) ExitAliasedRelation(ctx *AliasedRelationContext) {}

// EnterPivotClause is called when production pivotClause is entered.
func (s *BaseSqlBaseListener) EnterPivotClause(ctx *PivotClauseContext) {}

// ExitPivotClause is called when production pivotClause is exited.
func (s *BaseSqlBaseListener) ExitPivotClause(ctx *PivotClauseContext) {}

// EnterPivotArgs is called when production pivotArgs is entered.
func (s *BaseSqlBaseListener) EnterPivotArgs(ctx *PivotArgsContext) {}

// ExitPivotArgs is called when production pivotArgs is exited.
func (s *BaseSqlBaseListener) ExitPivotArgs(ctx *PivotArgsContext) {}

// EnterNamedValueExpression is called when production namedValueExpression is entered.
func (s *BaseSqlBaseListener) EnterNamedValueExpression(ctx *NamedValueExpressionContext) {}

// ExitNamedValueExpression is called when production namedValueExpression is exited.
func (s *BaseSqlBaseListener) ExitNamedValueExpression(ctx *NamedValueExpressionContext) {}

// EnterExpression is called when production expression is entered.
func (s *BaseSqlBaseListener) EnterExpression(ctx *ExpressionContext) {}

// ExitExpression is called when production expression is exited.
func (s *BaseSqlBaseListener) ExitExpression(ctx *ExpressionContext) {}

// EnterLogicalNot is called when production logicalNot is entered.
func (s *BaseSqlBaseListener) EnterLogicalNot(ctx *LogicalNotContext) {}

// ExitLogicalNot is called when production logicalNot is exited.
func (s *BaseSqlBaseListener) ExitLogicalNot(ctx *LogicalNotContext) {}

// EnterStringQuery is called when production stringQuery is entered.
func (s *BaseSqlBaseListener) EnterStringQuery(ctx *StringQueryContext) {}

// ExitStringQuery is called when production stringQuery is exited.
func (s *BaseSqlBaseListener) ExitStringQuery(ctx *StringQueryContext) {}

// EnterBooleanDefault is called when production booleanDefault is entered.
func (s *BaseSqlBaseListener) EnterBooleanDefault(ctx *BooleanDefaultContext) {}

// ExitBooleanDefault is called when production booleanDefault is exited.
func (s *BaseSqlBaseListener) ExitBooleanDefault(ctx *BooleanDefaultContext) {}

// EnterExists is called when production exists is entered.
func (s *BaseSqlBaseListener) EnterExists(ctx *ExistsContext) {}

// ExitExists is called when production exists is exited.
func (s *BaseSqlBaseListener) ExitExists(ctx *ExistsContext) {}

// EnterMultiMatchQuery is called when production multiMatchQuery is entered.
func (s *BaseSqlBaseListener) EnterMultiMatchQuery(ctx *MultiMatchQueryContext) {}

// ExitMultiMatchQuery is called when production multiMatchQuery is exited.
func (s *BaseSqlBaseListener) ExitMultiMatchQuery(ctx *MultiMatchQueryContext) {}

// EnterMatchQuery is called when production matchQuery is entered.
func (s *BaseSqlBaseListener) EnterMatchQuery(ctx *MatchQueryContext) {}

// ExitMatchQuery is called when production matchQuery is exited.
func (s *BaseSqlBaseListener) ExitMatchQuery(ctx *MatchQueryContext) {}

// EnterLogicalBinary is called when production logicalBinary is entered.
func (s *BaseSqlBaseListener) EnterLogicalBinary(ctx *LogicalBinaryContext) {}

// ExitLogicalBinary is called when production logicalBinary is exited.
func (s *BaseSqlBaseListener) ExitLogicalBinary(ctx *LogicalBinaryContext) {}

// EnterMatchQueryOptions is called when production matchQueryOptions is entered.
func (s *BaseSqlBaseListener) EnterMatchQueryOptions(ctx *MatchQueryOptionsContext) {}

// ExitMatchQueryOptions is called when production matchQueryOptions is exited.
func (s *BaseSqlBaseListener) ExitMatchQueryOptions(ctx *MatchQueryOptionsContext) {}

// EnterPredicated is called when production predicated is entered.
func (s *BaseSqlBaseListener) EnterPredicated(ctx *PredicatedContext) {}

// ExitPredicated is called when production predicated is exited.
func (s *BaseSqlBaseListener) ExitPredicated(ctx *PredicatedContext) {}

// EnterPredicate is called when production predicate is entered.
func (s *BaseSqlBaseListener) EnterPredicate(ctx *PredicateContext) {}

// ExitPredicate is called when production predicate is exited.
func (s *BaseSqlBaseListener) ExitPredicate(ctx *PredicateContext) {}

// EnterLikePattern is called when production likePattern is entered.
func (s *BaseSqlBaseListener) EnterLikePattern(ctx *LikePatternContext) {}

// ExitLikePattern is called when production likePattern is exited.
func (s *BaseSqlBaseListener) ExitLikePattern(ctx *LikePatternContext) {}

// EnterPattern is called when production pattern is entered.
func (s *BaseSqlBaseListener) EnterPattern(ctx *PatternContext) {}

// ExitPattern is called when production pattern is exited.
func (s *BaseSqlBaseListener) ExitPattern(ctx *PatternContext) {}

// EnterPatternEscape is called when production patternEscape is entered.
func (s *BaseSqlBaseListener) EnterPatternEscape(ctx *PatternEscapeContext) {}

// ExitPatternEscape is called when production patternEscape is exited.
func (s *BaseSqlBaseListener) ExitPatternEscape(ctx *PatternEscapeContext) {}

// EnterValueExpressionDefault is called when production valueExpressionDefault is entered.
func (s *BaseSqlBaseListener) EnterValueExpressionDefault(ctx *ValueExpressionDefaultContext) {}

// ExitValueExpressionDefault is called when production valueExpressionDefault is exited.
func (s *BaseSqlBaseListener) ExitValueExpressionDefault(ctx *ValueExpressionDefaultContext) {}

// EnterComparison is called when production comparison is entered.
func (s *BaseSqlBaseListener) EnterComparison(ctx *ComparisonContext) {}

// ExitComparison is called when production comparison is exited.
func (s *BaseSqlBaseListener) ExitComparison(ctx *ComparisonContext) {}

// EnterArithmeticBinary is called when production arithmeticBinary is entered.
func (s *BaseSqlBaseListener) EnterArithmeticBinary(ctx *ArithmeticBinaryContext) {}

// ExitArithmeticBinary is called when production arithmeticBinary is exited.
func (s *BaseSqlBaseListener) ExitArithmeticBinary(ctx *ArithmeticBinaryContext) {}

// EnterArithmeticUnary is called when production arithmeticUnary is entered.
func (s *BaseSqlBaseListener) EnterArithmeticUnary(ctx *ArithmeticUnaryContext) {}

// ExitArithmeticUnary is called when production arithmeticUnary is exited.
func (s *BaseSqlBaseListener) ExitArithmeticUnary(ctx *ArithmeticUnaryContext) {}

// EnterDereference is called when production dereference is entered.
func (s *BaseSqlBaseListener) EnterDereference(ctx *DereferenceContext) {}

// ExitDereference is called when production dereference is exited.
func (s *BaseSqlBaseListener) ExitDereference(ctx *DereferenceContext) {}

// EnterCast is called when production cast is entered.
func (s *BaseSqlBaseListener) EnterCast(ctx *CastContext) {}

// ExitCast is called when production cast is exited.
func (s *BaseSqlBaseListener) ExitCast(ctx *CastContext) {}

// EnterConstantDefault is called when production constantDefault is entered.
func (s *BaseSqlBaseListener) EnterConstantDefault(ctx *ConstantDefaultContext) {}

// ExitConstantDefault is called when production constantDefault is exited.
func (s *BaseSqlBaseListener) ExitConstantDefault(ctx *ConstantDefaultContext) {}

// EnterExtract is called when production extract is entered.
func (s *BaseSqlBaseListener) EnterExtract(ctx *ExtractContext) {}

// ExitExtract is called when production extract is exited.
func (s *BaseSqlBaseListener) ExitExtract(ctx *ExtractContext) {}

// EnterParenthesizedExpression is called when production parenthesizedExpression is entered.
func (s *BaseSqlBaseListener) EnterParenthesizedExpression(ctx *ParenthesizedExpressionContext) {}

// ExitParenthesizedExpression is called when production parenthesizedExpression is exited.
func (s *BaseSqlBaseListener) ExitParenthesizedExpression(ctx *ParenthesizedExpressionContext) {}

// EnterStar is called when production star is entered.
func (s *BaseSqlBaseListener) EnterStar(ctx *StarContext) {}

// ExitStar is called when production star is exited.
func (s *BaseSqlBaseListener) ExitStar(ctx *StarContext) {}

// EnterCastOperatorExpression is called when production castOperatorExpression is entered.
func (s *BaseSqlBaseListener) EnterCastOperatorExpression(ctx *CastOperatorExpressionContext) {}

// ExitCastOperatorExpression is called when production castOperatorExpression is exited.
func (s *BaseSqlBaseListener) ExitCastOperatorExpression(ctx *CastOperatorExpressionContext) {}

// EnterFunction is called when production function is entered.
func (s *BaseSqlBaseListener) EnterFunction(ctx *FunctionContext) {}

// ExitFunction is called when production function is exited.
func (s *BaseSqlBaseListener) ExitFunction(ctx *FunctionContext) {}

// EnterCurrentDateTimeFunction is called when production currentDateTimeFunction is entered.
func (s *BaseSqlBaseListener) EnterCurrentDateTimeFunction(ctx *CurrentDateTimeFunctionContext) {}

// ExitCurrentDateTimeFunction is called when production currentDateTimeFunction is exited.
func (s *BaseSqlBaseListener) ExitCurrentDateTimeFunction(ctx *CurrentDateTimeFunctionContext) {}

// EnterSubqueryExpression is called when production subqueryExpression is entered.
func (s *BaseSqlBaseListener) EnterSubqueryExpression(ctx *SubqueryExpressionContext) {}

// ExitSubqueryExpression is called when production subqueryExpression is exited.
func (s *BaseSqlBaseListener) ExitSubqueryExpression(ctx *SubqueryExpressionContext) {}

// EnterCase is called when production case is entered.
func (s *BaseSqlBaseListener) EnterCase(ctx *CaseContext) {}

// ExitCase is called when production case is exited.
func (s *BaseSqlBaseListener) ExitCase(ctx *CaseContext) {}

// EnterBuiltinDateTimeFunction is called when production builtinDateTimeFunction is entered.
func (s *BaseSqlBaseListener) EnterBuiltinDateTimeFunction(ctx *BuiltinDateTimeFunctionContext) {}

// ExitBuiltinDateTimeFunction is called when production builtinDateTimeFunction is exited.
func (s *BaseSqlBaseListener) ExitBuiltinDateTimeFunction(ctx *BuiltinDateTimeFunctionContext) {}

// EnterCastExpression is called when production castExpression is entered.
func (s *BaseSqlBaseListener) EnterCastExpression(ctx *CastExpressionContext) {}

// ExitCastExpression is called when production castExpression is exited.
func (s *BaseSqlBaseListener) ExitCastExpression(ctx *CastExpressionContext) {}

// EnterCastTemplate is called when production castTemplate is entered.
func (s *BaseSqlBaseListener) EnterCastTemplate(ctx *CastTemplateContext) {}

// ExitCastTemplate is called when production castTemplate is exited.
func (s *BaseSqlBaseListener) ExitCastTemplate(ctx *CastTemplateContext) {}

// EnterConvertTemplate is called when production convertTemplate is entered.
func (s *BaseSqlBaseListener) EnterConvertTemplate(ctx *ConvertTemplateContext) {}

// ExitConvertTemplate is called when production convertTemplate is exited.
func (s *BaseSqlBaseListener) ExitConvertTemplate(ctx *ConvertTemplateContext) {}

// EnterExtractExpression is called when production extractExpression is entered.
func (s *BaseSqlBaseListener) EnterExtractExpression(ctx *ExtractExpressionContext) {}

// ExitExtractExpression is called when production extractExpression is exited.
func (s *BaseSqlBaseListener) ExitExtractExpression(ctx *ExtractExpressionContext) {}

// EnterExtractTemplate is called when production extractTemplate is entered.
func (s *BaseSqlBaseListener) EnterExtractTemplate(ctx *ExtractTemplateContext) {}

// ExitExtractTemplate is called when production extractTemplate is exited.
func (s *BaseSqlBaseListener) ExitExtractTemplate(ctx *ExtractTemplateContext) {}

// EnterFunctionExpression is called when production functionExpression is entered.
func (s *BaseSqlBaseListener) EnterFunctionExpression(ctx *FunctionExpressionContext) {}

// ExitFunctionExpression is called when production functionExpression is exited.
func (s *BaseSqlBaseListener) ExitFunctionExpression(ctx *FunctionExpressionContext) {}

// EnterFunctionTemplate is called when production functionTemplate is entered.
func (s *BaseSqlBaseListener) EnterFunctionTemplate(ctx *FunctionTemplateContext) {}

// ExitFunctionTemplate is called when production functionTemplate is exited.
func (s *BaseSqlBaseListener) ExitFunctionTemplate(ctx *FunctionTemplateContext) {}

// EnterFunctionName is called when production functionName is entered.
func (s *BaseSqlBaseListener) EnterFunctionName(ctx *FunctionNameContext) {}

// ExitFunctionName is called when production functionName is exited.
func (s *BaseSqlBaseListener) ExitFunctionName(ctx *FunctionNameContext) {}

// EnterNullLiteral is called when production nullLiteral is entered.
func (s *BaseSqlBaseListener) EnterNullLiteral(ctx *NullLiteralContext) {}

// ExitNullLiteral is called when production nullLiteral is exited.
func (s *BaseSqlBaseListener) ExitNullLiteral(ctx *NullLiteralContext) {}

// EnterIntervalLiteral is called when production intervalLiteral is entered.
func (s *BaseSqlBaseListener) EnterIntervalLiteral(ctx *IntervalLiteralContext) {}

// ExitIntervalLiteral is called when production intervalLiteral is exited.
func (s *BaseSqlBaseListener) ExitIntervalLiteral(ctx *IntervalLiteralContext) {}

// EnterNumericLiteral is called when production numericLiteral is entered.
func (s *BaseSqlBaseListener) EnterNumericLiteral(ctx *NumericLiteralContext) {}

// ExitNumericLiteral is called when production numericLiteral is exited.
func (s *BaseSqlBaseListener) ExitNumericLiteral(ctx *NumericLiteralContext) {}

// EnterBooleanLiteral is called when production booleanLiteral is entered.
func (s *BaseSqlBaseListener) EnterBooleanLiteral(ctx *BooleanLiteralContext) {}

// ExitBooleanLiteral is called when production booleanLiteral is exited.
func (s *BaseSqlBaseListener) ExitBooleanLiteral(ctx *BooleanLiteralContext) {}

// EnterStringLiteral is called when production stringLiteral is entered.
func (s *BaseSqlBaseListener) EnterStringLiteral(ctx *StringLiteralContext) {}

// ExitStringLiteral is called when production stringLiteral is exited.
func (s *BaseSqlBaseListener) ExitStringLiteral(ctx *StringLiteralContext) {}

// EnterParamLiteral is called when production paramLiteral is entered.
func (s *BaseSqlBaseListener) EnterParamLiteral(ctx *ParamLiteralContext) {}

// ExitParamLiteral is called when production paramLiteral is exited.
func (s *BaseSqlBaseListener) ExitParamLiteral(ctx *ParamLiteralContext) {}

// EnterDateEscapedLiteral is called when production dateEscapedLiteral is entered.
func (s *BaseSqlBaseListener) EnterDateEscapedLiteral(ctx *DateEscapedLiteralContext) {}

// ExitDateEscapedLiteral is called when production dateEscapedLiteral is exited.
func (s *BaseSqlBaseListener) ExitDateEscapedLiteral(ctx *DateEscapedLiteralContext) {}

// EnterTimeEscapedLiteral is called when production timeEscapedLiteral is entered.
func (s *BaseSqlBaseListener) EnterTimeEscapedLiteral(ctx *TimeEscapedLiteralContext) {}

// ExitTimeEscapedLiteral is called when production timeEscapedLiteral is exited.
func (s *BaseSqlBaseListener) ExitTimeEscapedLiteral(ctx *TimeEscapedLiteralContext) {}

// EnterTimestampEscapedLiteral is called when production timestampEscapedLiteral is entered.
func (s *BaseSqlBaseListener) EnterTimestampEscapedLiteral(ctx *TimestampEscapedLiteralContext) {}

// ExitTimestampEscapedLiteral is called when production timestampEscapedLiteral is exited.
func (s *BaseSqlBaseListener) ExitTimestampEscapedLiteral(ctx *TimestampEscapedLiteralContext) {}

// EnterGuidEscapedLiteral is called when production guidEscapedLiteral is entered.
func (s *BaseSqlBaseListener) EnterGuidEscapedLiteral(ctx *GuidEscapedLiteralContext) {}

// ExitGuidEscapedLiteral is called when production guidEscapedLiteral is exited.
func (s *BaseSqlBaseListener) ExitGuidEscapedLiteral(ctx *GuidEscapedLiteralContext) {}

// EnterComparisonOperator is called when production comparisonOperator is entered.
func (s *BaseSqlBaseListener) EnterComparisonOperator(ctx *ComparisonOperatorContext) {}

// ExitComparisonOperator is called when production comparisonOperator is exited.
func (s *BaseSqlBaseListener) ExitComparisonOperator(ctx *ComparisonOperatorContext) {}

// EnterBooleanValue is called when production booleanValue is entered.
func (s *BaseSqlBaseListener) EnterBooleanValue(ctx *BooleanValueContext) {}

// ExitBooleanValue is called when production booleanValue is exited.
func (s *BaseSqlBaseListener) ExitBooleanValue(ctx *BooleanValueContext) {}

// EnterInterval is called when production interval is entered.
func (s *BaseSqlBaseListener) EnterInterval(ctx *IntervalContext) {}

// ExitInterval is called when production interval is exited.
func (s *BaseSqlBaseListener) ExitInterval(ctx *IntervalContext) {}

// EnterIntervalField is called when production intervalField is entered.
func (s *BaseSqlBaseListener) EnterIntervalField(ctx *IntervalFieldContext) {}

// ExitIntervalField is called when production intervalField is exited.
func (s *BaseSqlBaseListener) ExitIntervalField(ctx *IntervalFieldContext) {}

// EnterPrimitiveDataType is called when production primitiveDataType is entered.
func (s *BaseSqlBaseListener) EnterPrimitiveDataType(ctx *PrimitiveDataTypeContext) {}

// ExitPrimitiveDataType is called when production primitiveDataType is exited.
func (s *BaseSqlBaseListener) ExitPrimitiveDataType(ctx *PrimitiveDataTypeContext) {}

// EnterQualifiedName is called when production qualifiedName is entered.
func (s *BaseSqlBaseListener) EnterQualifiedName(ctx *QualifiedNameContext) {}

// ExitQualifiedName is called when production qualifiedName is exited.
func (s *BaseSqlBaseListener) ExitQualifiedName(ctx *QualifiedNameContext) {}

// EnterIdentifier is called when production identifier is entered.
func (s *BaseSqlBaseListener) EnterIdentifier(ctx *IdentifierContext) {}

// ExitIdentifier is called when production identifier is exited.
func (s *BaseSqlBaseListener) ExitIdentifier(ctx *IdentifierContext) {}

// EnterTableIdentifier is called when production tableIdentifier is entered.
func (s *BaseSqlBaseListener) EnterTableIdentifier(ctx *TableIdentifierContext) {}

// ExitTableIdentifier is called when production tableIdentifier is exited.
func (s *BaseSqlBaseListener) ExitTableIdentifier(ctx *TableIdentifierContext) {}

// EnterQuotedIdentifier is called when production quotedIdentifier is entered.
func (s *BaseSqlBaseListener) EnterQuotedIdentifier(ctx *QuotedIdentifierContext) {}

// ExitQuotedIdentifier is called when production quotedIdentifier is exited.
func (s *BaseSqlBaseListener) ExitQuotedIdentifier(ctx *QuotedIdentifierContext) {}

// EnterBackQuotedIdentifier is called when production backQuotedIdentifier is entered.
func (s *BaseSqlBaseListener) EnterBackQuotedIdentifier(ctx *BackQuotedIdentifierContext) {}

// ExitBackQuotedIdentifier is called when production backQuotedIdentifier is exited.
func (s *BaseSqlBaseListener) ExitBackQuotedIdentifier(ctx *BackQuotedIdentifierContext) {}

// EnterUnquotedIdentifier is called when production unquotedIdentifier is entered.
func (s *BaseSqlBaseListener) EnterUnquotedIdentifier(ctx *UnquotedIdentifierContext) {}

// ExitUnquotedIdentifier is called when production unquotedIdentifier is exited.
func (s *BaseSqlBaseListener) ExitUnquotedIdentifier(ctx *UnquotedIdentifierContext) {}

// EnterDigitIdentifier is called when production digitIdentifier is entered.
func (s *BaseSqlBaseListener) EnterDigitIdentifier(ctx *DigitIdentifierContext) {}

// ExitDigitIdentifier is called when production digitIdentifier is exited.
func (s *BaseSqlBaseListener) ExitDigitIdentifier(ctx *DigitIdentifierContext) {}

// EnterDecimalLiteral is called when production decimalLiteral is entered.
func (s *BaseSqlBaseListener) EnterDecimalLiteral(ctx *DecimalLiteralContext) {}

// ExitDecimalLiteral is called when production decimalLiteral is exited.
func (s *BaseSqlBaseListener) ExitDecimalLiteral(ctx *DecimalLiteralContext) {}

// EnterIntegerLiteral is called when production integerLiteral is entered.
func (s *BaseSqlBaseListener) EnterIntegerLiteral(ctx *IntegerLiteralContext) {}

// ExitIntegerLiteral is called when production integerLiteral is exited.
func (s *BaseSqlBaseListener) ExitIntegerLiteral(ctx *IntegerLiteralContext) {}

// EnterString is called when production string is entered.
func (s *BaseSqlBaseListener) EnterString(ctx *StringContext) {}

// ExitString is called when production string is exited.
func (s *BaseSqlBaseListener) ExitString(ctx *StringContext) {}

// EnterWhenClause is called when production whenClause is entered.
func (s *BaseSqlBaseListener) EnterWhenClause(ctx *WhenClauseContext) {}

// ExitWhenClause is called when production whenClause is exited.
func (s *BaseSqlBaseListener) ExitWhenClause(ctx *WhenClauseContext) {}

// EnterNonReserved is called when production nonReserved is entered.
func (s *BaseSqlBaseListener) EnterNonReserved(ctx *NonReservedContext) {}

// ExitNonReserved is called when production nonReserved is exited.
func (s *BaseSqlBaseListener) ExitNonReserved(ctx *NonReservedContext) {}
