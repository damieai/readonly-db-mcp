// Code generated from tools/es-language-parser/upstream/esql8/EsqlBaseParser.g4 by ANTLR 4.13.1. DO NOT EDIT.

package v8 // EsqlBaseParser
import "github.com/antlr4-go/antlr/v4"

// BaseEsqlBaseParserListener is a complete listener for a parse tree produced by EsqlBaseParser.
type BaseEsqlBaseParserListener struct{}

var _ EsqlBaseParserListener = &BaseEsqlBaseParserListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseEsqlBaseParserListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseEsqlBaseParserListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseEsqlBaseParserListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseEsqlBaseParserListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterSingleStatement is called when production singleStatement is entered.
func (s *BaseEsqlBaseParserListener) EnterSingleStatement(ctx *SingleStatementContext) {}

// ExitSingleStatement is called when production singleStatement is exited.
func (s *BaseEsqlBaseParserListener) ExitSingleStatement(ctx *SingleStatementContext) {}

// EnterCompositeQuery is called when production compositeQuery is entered.
func (s *BaseEsqlBaseParserListener) EnterCompositeQuery(ctx *CompositeQueryContext) {}

// ExitCompositeQuery is called when production compositeQuery is exited.
func (s *BaseEsqlBaseParserListener) ExitCompositeQuery(ctx *CompositeQueryContext) {}

// EnterSingleCommandQuery is called when production singleCommandQuery is entered.
func (s *BaseEsqlBaseParserListener) EnterSingleCommandQuery(ctx *SingleCommandQueryContext) {}

// ExitSingleCommandQuery is called when production singleCommandQuery is exited.
func (s *BaseEsqlBaseParserListener) ExitSingleCommandQuery(ctx *SingleCommandQueryContext) {}

// EnterSourceCommand is called when production sourceCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterSourceCommand(ctx *SourceCommandContext) {}

// ExitSourceCommand is called when production sourceCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitSourceCommand(ctx *SourceCommandContext) {}

// EnterProcessingCommand is called when production processingCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterProcessingCommand(ctx *ProcessingCommandContext) {}

// ExitProcessingCommand is called when production processingCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitProcessingCommand(ctx *ProcessingCommandContext) {}

// EnterWhereCommand is called when production whereCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterWhereCommand(ctx *WhereCommandContext) {}

// ExitWhereCommand is called when production whereCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitWhereCommand(ctx *WhereCommandContext) {}

// EnterMatchExpression is called when production matchExpression is entered.
func (s *BaseEsqlBaseParserListener) EnterMatchExpression(ctx *MatchExpressionContext) {}

// ExitMatchExpression is called when production matchExpression is exited.
func (s *BaseEsqlBaseParserListener) ExitMatchExpression(ctx *MatchExpressionContext) {}

// EnterLogicalNot is called when production logicalNot is entered.
func (s *BaseEsqlBaseParserListener) EnterLogicalNot(ctx *LogicalNotContext) {}

// ExitLogicalNot is called when production logicalNot is exited.
func (s *BaseEsqlBaseParserListener) ExitLogicalNot(ctx *LogicalNotContext) {}

// EnterBooleanDefault is called when production booleanDefault is entered.
func (s *BaseEsqlBaseParserListener) EnterBooleanDefault(ctx *BooleanDefaultContext) {}

// ExitBooleanDefault is called when production booleanDefault is exited.
func (s *BaseEsqlBaseParserListener) ExitBooleanDefault(ctx *BooleanDefaultContext) {}

// EnterIsNull is called when production isNull is entered.
func (s *BaseEsqlBaseParserListener) EnterIsNull(ctx *IsNullContext) {}

// ExitIsNull is called when production isNull is exited.
func (s *BaseEsqlBaseParserListener) ExitIsNull(ctx *IsNullContext) {}

// EnterRegexExpression is called when production regexExpression is entered.
func (s *BaseEsqlBaseParserListener) EnterRegexExpression(ctx *RegexExpressionContext) {}

// ExitRegexExpression is called when production regexExpression is exited.
func (s *BaseEsqlBaseParserListener) ExitRegexExpression(ctx *RegexExpressionContext) {}

// EnterLogicalIn is called when production logicalIn is entered.
func (s *BaseEsqlBaseParserListener) EnterLogicalIn(ctx *LogicalInContext) {}

// ExitLogicalIn is called when production logicalIn is exited.
func (s *BaseEsqlBaseParserListener) ExitLogicalIn(ctx *LogicalInContext) {}

// EnterLogicalBinary is called when production logicalBinary is entered.
func (s *BaseEsqlBaseParserListener) EnterLogicalBinary(ctx *LogicalBinaryContext) {}

// ExitLogicalBinary is called when production logicalBinary is exited.
func (s *BaseEsqlBaseParserListener) ExitLogicalBinary(ctx *LogicalBinaryContext) {}

// EnterLikeExpression is called when production likeExpression is entered.
func (s *BaseEsqlBaseParserListener) EnterLikeExpression(ctx *LikeExpressionContext) {}

// ExitLikeExpression is called when production likeExpression is exited.
func (s *BaseEsqlBaseParserListener) ExitLikeExpression(ctx *LikeExpressionContext) {}

// EnterRlikeExpression is called when production rlikeExpression is entered.
func (s *BaseEsqlBaseParserListener) EnterRlikeExpression(ctx *RlikeExpressionContext) {}

// ExitRlikeExpression is called when production rlikeExpression is exited.
func (s *BaseEsqlBaseParserListener) ExitRlikeExpression(ctx *RlikeExpressionContext) {}

// EnterLikeListExpression is called when production likeListExpression is entered.
func (s *BaseEsqlBaseParserListener) EnterLikeListExpression(ctx *LikeListExpressionContext) {}

// ExitLikeListExpression is called when production likeListExpression is exited.
func (s *BaseEsqlBaseParserListener) ExitLikeListExpression(ctx *LikeListExpressionContext) {}

// EnterMatchBooleanExpression is called when production matchBooleanExpression is entered.
func (s *BaseEsqlBaseParserListener) EnterMatchBooleanExpression(ctx *MatchBooleanExpressionContext) {
}

// ExitMatchBooleanExpression is called when production matchBooleanExpression is exited.
func (s *BaseEsqlBaseParserListener) ExitMatchBooleanExpression(ctx *MatchBooleanExpressionContext) {}

// EnterValueExpressionDefault is called when production valueExpressionDefault is entered.
func (s *BaseEsqlBaseParserListener) EnterValueExpressionDefault(ctx *ValueExpressionDefaultContext) {
}

// ExitValueExpressionDefault is called when production valueExpressionDefault is exited.
func (s *BaseEsqlBaseParserListener) ExitValueExpressionDefault(ctx *ValueExpressionDefaultContext) {}

// EnterComparison is called when production comparison is entered.
func (s *BaseEsqlBaseParserListener) EnterComparison(ctx *ComparisonContext) {}

// ExitComparison is called when production comparison is exited.
func (s *BaseEsqlBaseParserListener) ExitComparison(ctx *ComparisonContext) {}

// EnterOperatorExpressionDefault is called when production operatorExpressionDefault is entered.
func (s *BaseEsqlBaseParserListener) EnterOperatorExpressionDefault(ctx *OperatorExpressionDefaultContext) {
}

// ExitOperatorExpressionDefault is called when production operatorExpressionDefault is exited.
func (s *BaseEsqlBaseParserListener) ExitOperatorExpressionDefault(ctx *OperatorExpressionDefaultContext) {
}

// EnterArithmeticBinary is called when production arithmeticBinary is entered.
func (s *BaseEsqlBaseParserListener) EnterArithmeticBinary(ctx *ArithmeticBinaryContext) {}

// ExitArithmeticBinary is called when production arithmeticBinary is exited.
func (s *BaseEsqlBaseParserListener) ExitArithmeticBinary(ctx *ArithmeticBinaryContext) {}

// EnterArithmeticUnary is called when production arithmeticUnary is entered.
func (s *BaseEsqlBaseParserListener) EnterArithmeticUnary(ctx *ArithmeticUnaryContext) {}

// ExitArithmeticUnary is called when production arithmeticUnary is exited.
func (s *BaseEsqlBaseParserListener) ExitArithmeticUnary(ctx *ArithmeticUnaryContext) {}

// EnterDereference is called when production dereference is entered.
func (s *BaseEsqlBaseParserListener) EnterDereference(ctx *DereferenceContext) {}

// ExitDereference is called when production dereference is exited.
func (s *BaseEsqlBaseParserListener) ExitDereference(ctx *DereferenceContext) {}

// EnterInlineCast is called when production inlineCast is entered.
func (s *BaseEsqlBaseParserListener) EnterInlineCast(ctx *InlineCastContext) {}

// ExitInlineCast is called when production inlineCast is exited.
func (s *BaseEsqlBaseParserListener) ExitInlineCast(ctx *InlineCastContext) {}

// EnterConstantDefault is called when production constantDefault is entered.
func (s *BaseEsqlBaseParserListener) EnterConstantDefault(ctx *ConstantDefaultContext) {}

// ExitConstantDefault is called when production constantDefault is exited.
func (s *BaseEsqlBaseParserListener) ExitConstantDefault(ctx *ConstantDefaultContext) {}

// EnterParenthesizedExpression is called when production parenthesizedExpression is entered.
func (s *BaseEsqlBaseParserListener) EnterParenthesizedExpression(ctx *ParenthesizedExpressionContext) {
}

// ExitParenthesizedExpression is called when production parenthesizedExpression is exited.
func (s *BaseEsqlBaseParserListener) ExitParenthesizedExpression(ctx *ParenthesizedExpressionContext) {
}

// EnterFunction is called when production function is entered.
func (s *BaseEsqlBaseParserListener) EnterFunction(ctx *FunctionContext) {}

// ExitFunction is called when production function is exited.
func (s *BaseEsqlBaseParserListener) ExitFunction(ctx *FunctionContext) {}

// EnterFunctionExpression is called when production functionExpression is entered.
func (s *BaseEsqlBaseParserListener) EnterFunctionExpression(ctx *FunctionExpressionContext) {}

// ExitFunctionExpression is called when production functionExpression is exited.
func (s *BaseEsqlBaseParserListener) ExitFunctionExpression(ctx *FunctionExpressionContext) {}

// EnterFunctionName is called when production functionName is entered.
func (s *BaseEsqlBaseParserListener) EnterFunctionName(ctx *FunctionNameContext) {}

// ExitFunctionName is called when production functionName is exited.
func (s *BaseEsqlBaseParserListener) ExitFunctionName(ctx *FunctionNameContext) {}

// EnterMapExpression is called when production mapExpression is entered.
func (s *BaseEsqlBaseParserListener) EnterMapExpression(ctx *MapExpressionContext) {}

// ExitMapExpression is called when production mapExpression is exited.
func (s *BaseEsqlBaseParserListener) ExitMapExpression(ctx *MapExpressionContext) {}

// EnterEntryExpression is called when production entryExpression is entered.
func (s *BaseEsqlBaseParserListener) EnterEntryExpression(ctx *EntryExpressionContext) {}

// ExitEntryExpression is called when production entryExpression is exited.
func (s *BaseEsqlBaseParserListener) ExitEntryExpression(ctx *EntryExpressionContext) {}

// EnterToDataType is called when production toDataType is entered.
func (s *BaseEsqlBaseParserListener) EnterToDataType(ctx *ToDataTypeContext) {}

// ExitToDataType is called when production toDataType is exited.
func (s *BaseEsqlBaseParserListener) ExitToDataType(ctx *ToDataTypeContext) {}

// EnterRowCommand is called when production rowCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterRowCommand(ctx *RowCommandContext) {}

// ExitRowCommand is called when production rowCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitRowCommand(ctx *RowCommandContext) {}

// EnterFields is called when production fields is entered.
func (s *BaseEsqlBaseParserListener) EnterFields(ctx *FieldsContext) {}

// ExitFields is called when production fields is exited.
func (s *BaseEsqlBaseParserListener) ExitFields(ctx *FieldsContext) {}

// EnterField is called when production field is entered.
func (s *BaseEsqlBaseParserListener) EnterField(ctx *FieldContext) {}

// ExitField is called when production field is exited.
func (s *BaseEsqlBaseParserListener) ExitField(ctx *FieldContext) {}

// EnterRerankFields is called when production rerankFields is entered.
func (s *BaseEsqlBaseParserListener) EnterRerankFields(ctx *RerankFieldsContext) {}

// ExitRerankFields is called when production rerankFields is exited.
func (s *BaseEsqlBaseParserListener) ExitRerankFields(ctx *RerankFieldsContext) {}

// EnterRerankField is called when production rerankField is entered.
func (s *BaseEsqlBaseParserListener) EnterRerankField(ctx *RerankFieldContext) {}

// ExitRerankField is called when production rerankField is exited.
func (s *BaseEsqlBaseParserListener) ExitRerankField(ctx *RerankFieldContext) {}

// EnterFromCommand is called when production fromCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterFromCommand(ctx *FromCommandContext) {}

// ExitFromCommand is called when production fromCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitFromCommand(ctx *FromCommandContext) {}

// EnterIndexPattern is called when production indexPattern is entered.
func (s *BaseEsqlBaseParserListener) EnterIndexPattern(ctx *IndexPatternContext) {}

// ExitIndexPattern is called when production indexPattern is exited.
func (s *BaseEsqlBaseParserListener) ExitIndexPattern(ctx *IndexPatternContext) {}

// EnterClusterString is called when production clusterString is entered.
func (s *BaseEsqlBaseParserListener) EnterClusterString(ctx *ClusterStringContext) {}

// ExitClusterString is called when production clusterString is exited.
func (s *BaseEsqlBaseParserListener) ExitClusterString(ctx *ClusterStringContext) {}

// EnterSelectorString is called when production selectorString is entered.
func (s *BaseEsqlBaseParserListener) EnterSelectorString(ctx *SelectorStringContext) {}

// ExitSelectorString is called when production selectorString is exited.
func (s *BaseEsqlBaseParserListener) ExitSelectorString(ctx *SelectorStringContext) {}

// EnterUnquotedIndexString is called when production unquotedIndexString is entered.
func (s *BaseEsqlBaseParserListener) EnterUnquotedIndexString(ctx *UnquotedIndexStringContext) {}

// ExitUnquotedIndexString is called when production unquotedIndexString is exited.
func (s *BaseEsqlBaseParserListener) ExitUnquotedIndexString(ctx *UnquotedIndexStringContext) {}

// EnterIndexString is called when production indexString is entered.
func (s *BaseEsqlBaseParserListener) EnterIndexString(ctx *IndexStringContext) {}

// ExitIndexString is called when production indexString is exited.
func (s *BaseEsqlBaseParserListener) ExitIndexString(ctx *IndexStringContext) {}

// EnterMetadata is called when production metadata is entered.
func (s *BaseEsqlBaseParserListener) EnterMetadata(ctx *MetadataContext) {}

// ExitMetadata is called when production metadata is exited.
func (s *BaseEsqlBaseParserListener) ExitMetadata(ctx *MetadataContext) {}

// EnterMetadataOption is called when production metadataOption is entered.
func (s *BaseEsqlBaseParserListener) EnterMetadataOption(ctx *MetadataOptionContext) {}

// ExitMetadataOption is called when production metadataOption is exited.
func (s *BaseEsqlBaseParserListener) ExitMetadataOption(ctx *MetadataOptionContext) {}

// EnterDeprecated_metadata is called when production deprecated_metadata is entered.
func (s *BaseEsqlBaseParserListener) EnterDeprecated_metadata(ctx *Deprecated_metadataContext) {}

// ExitDeprecated_metadata is called when production deprecated_metadata is exited.
func (s *BaseEsqlBaseParserListener) ExitDeprecated_metadata(ctx *Deprecated_metadataContext) {}

// EnterMetricsCommand is called when production metricsCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterMetricsCommand(ctx *MetricsCommandContext) {}

// ExitMetricsCommand is called when production metricsCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitMetricsCommand(ctx *MetricsCommandContext) {}

// EnterEvalCommand is called when production evalCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterEvalCommand(ctx *EvalCommandContext) {}

// ExitEvalCommand is called when production evalCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitEvalCommand(ctx *EvalCommandContext) {}

// EnterStatsCommand is called when production statsCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterStatsCommand(ctx *StatsCommandContext) {}

// ExitStatsCommand is called when production statsCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitStatsCommand(ctx *StatsCommandContext) {}

// EnterAggFields is called when production aggFields is entered.
func (s *BaseEsqlBaseParserListener) EnterAggFields(ctx *AggFieldsContext) {}

// ExitAggFields is called when production aggFields is exited.
func (s *BaseEsqlBaseParserListener) ExitAggFields(ctx *AggFieldsContext) {}

// EnterAggField is called when production aggField is entered.
func (s *BaseEsqlBaseParserListener) EnterAggField(ctx *AggFieldContext) {}

// ExitAggField is called when production aggField is exited.
func (s *BaseEsqlBaseParserListener) ExitAggField(ctx *AggFieldContext) {}

// EnterQualifiedName is called when production qualifiedName is entered.
func (s *BaseEsqlBaseParserListener) EnterQualifiedName(ctx *QualifiedNameContext) {}

// ExitQualifiedName is called when production qualifiedName is exited.
func (s *BaseEsqlBaseParserListener) ExitQualifiedName(ctx *QualifiedNameContext) {}

// EnterQualifiedNamePattern is called when production qualifiedNamePattern is entered.
func (s *BaseEsqlBaseParserListener) EnterQualifiedNamePattern(ctx *QualifiedNamePatternContext) {}

// ExitQualifiedNamePattern is called when production qualifiedNamePattern is exited.
func (s *BaseEsqlBaseParserListener) ExitQualifiedNamePattern(ctx *QualifiedNamePatternContext) {}

// EnterQualifiedNamePatterns is called when production qualifiedNamePatterns is entered.
func (s *BaseEsqlBaseParserListener) EnterQualifiedNamePatterns(ctx *QualifiedNamePatternsContext) {}

// ExitQualifiedNamePatterns is called when production qualifiedNamePatterns is exited.
func (s *BaseEsqlBaseParserListener) ExitQualifiedNamePatterns(ctx *QualifiedNamePatternsContext) {}

// EnterIdentifier is called when production identifier is entered.
func (s *BaseEsqlBaseParserListener) EnterIdentifier(ctx *IdentifierContext) {}

// ExitIdentifier is called when production identifier is exited.
func (s *BaseEsqlBaseParserListener) ExitIdentifier(ctx *IdentifierContext) {}

// EnterIdentifierPattern is called when production identifierPattern is entered.
func (s *BaseEsqlBaseParserListener) EnterIdentifierPattern(ctx *IdentifierPatternContext) {}

// ExitIdentifierPattern is called when production identifierPattern is exited.
func (s *BaseEsqlBaseParserListener) ExitIdentifierPattern(ctx *IdentifierPatternContext) {}

// EnterNullLiteral is called when production nullLiteral is entered.
func (s *BaseEsqlBaseParserListener) EnterNullLiteral(ctx *NullLiteralContext) {}

// ExitNullLiteral is called when production nullLiteral is exited.
func (s *BaseEsqlBaseParserListener) ExitNullLiteral(ctx *NullLiteralContext) {}

// EnterQualifiedIntegerLiteral is called when production qualifiedIntegerLiteral is entered.
func (s *BaseEsqlBaseParserListener) EnterQualifiedIntegerLiteral(ctx *QualifiedIntegerLiteralContext) {
}

// ExitQualifiedIntegerLiteral is called when production qualifiedIntegerLiteral is exited.
func (s *BaseEsqlBaseParserListener) ExitQualifiedIntegerLiteral(ctx *QualifiedIntegerLiteralContext) {
}

// EnterDecimalLiteral is called when production decimalLiteral is entered.
func (s *BaseEsqlBaseParserListener) EnterDecimalLiteral(ctx *DecimalLiteralContext) {}

// ExitDecimalLiteral is called when production decimalLiteral is exited.
func (s *BaseEsqlBaseParserListener) ExitDecimalLiteral(ctx *DecimalLiteralContext) {}

// EnterIntegerLiteral is called when production integerLiteral is entered.
func (s *BaseEsqlBaseParserListener) EnterIntegerLiteral(ctx *IntegerLiteralContext) {}

// ExitIntegerLiteral is called when production integerLiteral is exited.
func (s *BaseEsqlBaseParserListener) ExitIntegerLiteral(ctx *IntegerLiteralContext) {}

// EnterBooleanLiteral is called when production booleanLiteral is entered.
func (s *BaseEsqlBaseParserListener) EnterBooleanLiteral(ctx *BooleanLiteralContext) {}

// ExitBooleanLiteral is called when production booleanLiteral is exited.
func (s *BaseEsqlBaseParserListener) ExitBooleanLiteral(ctx *BooleanLiteralContext) {}

// EnterInputParameter is called when production inputParameter is entered.
func (s *BaseEsqlBaseParserListener) EnterInputParameter(ctx *InputParameterContext) {}

// ExitInputParameter is called when production inputParameter is exited.
func (s *BaseEsqlBaseParserListener) ExitInputParameter(ctx *InputParameterContext) {}

// EnterStringLiteral is called when production stringLiteral is entered.
func (s *BaseEsqlBaseParserListener) EnterStringLiteral(ctx *StringLiteralContext) {}

// ExitStringLiteral is called when production stringLiteral is exited.
func (s *BaseEsqlBaseParserListener) ExitStringLiteral(ctx *StringLiteralContext) {}

// EnterNumericArrayLiteral is called when production numericArrayLiteral is entered.
func (s *BaseEsqlBaseParserListener) EnterNumericArrayLiteral(ctx *NumericArrayLiteralContext) {}

// ExitNumericArrayLiteral is called when production numericArrayLiteral is exited.
func (s *BaseEsqlBaseParserListener) ExitNumericArrayLiteral(ctx *NumericArrayLiteralContext) {}

// EnterBooleanArrayLiteral is called when production booleanArrayLiteral is entered.
func (s *BaseEsqlBaseParserListener) EnterBooleanArrayLiteral(ctx *BooleanArrayLiteralContext) {}

// ExitBooleanArrayLiteral is called when production booleanArrayLiteral is exited.
func (s *BaseEsqlBaseParserListener) ExitBooleanArrayLiteral(ctx *BooleanArrayLiteralContext) {}

// EnterStringArrayLiteral is called when production stringArrayLiteral is entered.
func (s *BaseEsqlBaseParserListener) EnterStringArrayLiteral(ctx *StringArrayLiteralContext) {}

// ExitStringArrayLiteral is called when production stringArrayLiteral is exited.
func (s *BaseEsqlBaseParserListener) ExitStringArrayLiteral(ctx *StringArrayLiteralContext) {}

// EnterInputParam is called when production inputParam is entered.
func (s *BaseEsqlBaseParserListener) EnterInputParam(ctx *InputParamContext) {}

// ExitInputParam is called when production inputParam is exited.
func (s *BaseEsqlBaseParserListener) ExitInputParam(ctx *InputParamContext) {}

// EnterInputNamedOrPositionalParam is called when production inputNamedOrPositionalParam is entered.
func (s *BaseEsqlBaseParserListener) EnterInputNamedOrPositionalParam(ctx *InputNamedOrPositionalParamContext) {
}

// ExitInputNamedOrPositionalParam is called when production inputNamedOrPositionalParam is exited.
func (s *BaseEsqlBaseParserListener) ExitInputNamedOrPositionalParam(ctx *InputNamedOrPositionalParamContext) {
}

// EnterInputDoubleParams is called when production inputDoubleParams is entered.
func (s *BaseEsqlBaseParserListener) EnterInputDoubleParams(ctx *InputDoubleParamsContext) {}

// ExitInputDoubleParams is called when production inputDoubleParams is exited.
func (s *BaseEsqlBaseParserListener) ExitInputDoubleParams(ctx *InputDoubleParamsContext) {}

// EnterInputNamedOrPositionalDoubleParams is called when production inputNamedOrPositionalDoubleParams is entered.
func (s *BaseEsqlBaseParserListener) EnterInputNamedOrPositionalDoubleParams(ctx *InputNamedOrPositionalDoubleParamsContext) {
}

// ExitInputNamedOrPositionalDoubleParams is called when production inputNamedOrPositionalDoubleParams is exited.
func (s *BaseEsqlBaseParserListener) ExitInputNamedOrPositionalDoubleParams(ctx *InputNamedOrPositionalDoubleParamsContext) {
}

// EnterIdentifierOrParameter is called when production identifierOrParameter is entered.
func (s *BaseEsqlBaseParserListener) EnterIdentifierOrParameter(ctx *IdentifierOrParameterContext) {}

// ExitIdentifierOrParameter is called when production identifierOrParameter is exited.
func (s *BaseEsqlBaseParserListener) ExitIdentifierOrParameter(ctx *IdentifierOrParameterContext) {}

// EnterLimitCommand is called when production limitCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterLimitCommand(ctx *LimitCommandContext) {}

// ExitLimitCommand is called when production limitCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitLimitCommand(ctx *LimitCommandContext) {}

// EnterSortCommand is called when production sortCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterSortCommand(ctx *SortCommandContext) {}

// ExitSortCommand is called when production sortCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitSortCommand(ctx *SortCommandContext) {}

// EnterOrderExpression is called when production orderExpression is entered.
func (s *BaseEsqlBaseParserListener) EnterOrderExpression(ctx *OrderExpressionContext) {}

// ExitOrderExpression is called when production orderExpression is exited.
func (s *BaseEsqlBaseParserListener) ExitOrderExpression(ctx *OrderExpressionContext) {}

// EnterKeepCommand is called when production keepCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterKeepCommand(ctx *KeepCommandContext) {}

// ExitKeepCommand is called when production keepCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitKeepCommand(ctx *KeepCommandContext) {}

// EnterDropCommand is called when production dropCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterDropCommand(ctx *DropCommandContext) {}

// ExitDropCommand is called when production dropCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitDropCommand(ctx *DropCommandContext) {}

// EnterRenameCommand is called when production renameCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterRenameCommand(ctx *RenameCommandContext) {}

// ExitRenameCommand is called when production renameCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitRenameCommand(ctx *RenameCommandContext) {}

// EnterRenameClause is called when production renameClause is entered.
func (s *BaseEsqlBaseParserListener) EnterRenameClause(ctx *RenameClauseContext) {}

// ExitRenameClause is called when production renameClause is exited.
func (s *BaseEsqlBaseParserListener) ExitRenameClause(ctx *RenameClauseContext) {}

// EnterDissectCommand is called when production dissectCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterDissectCommand(ctx *DissectCommandContext) {}

// ExitDissectCommand is called when production dissectCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitDissectCommand(ctx *DissectCommandContext) {}

// EnterGrokCommand is called when production grokCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterGrokCommand(ctx *GrokCommandContext) {}

// ExitGrokCommand is called when production grokCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitGrokCommand(ctx *GrokCommandContext) {}

// EnterMvExpandCommand is called when production mvExpandCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterMvExpandCommand(ctx *MvExpandCommandContext) {}

// ExitMvExpandCommand is called when production mvExpandCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitMvExpandCommand(ctx *MvExpandCommandContext) {}

// EnterCommandOptions is called when production commandOptions is entered.
func (s *BaseEsqlBaseParserListener) EnterCommandOptions(ctx *CommandOptionsContext) {}

// ExitCommandOptions is called when production commandOptions is exited.
func (s *BaseEsqlBaseParserListener) ExitCommandOptions(ctx *CommandOptionsContext) {}

// EnterCommandOption is called when production commandOption is entered.
func (s *BaseEsqlBaseParserListener) EnterCommandOption(ctx *CommandOptionContext) {}

// ExitCommandOption is called when production commandOption is exited.
func (s *BaseEsqlBaseParserListener) ExitCommandOption(ctx *CommandOptionContext) {}

// EnterBooleanValue is called when production booleanValue is entered.
func (s *BaseEsqlBaseParserListener) EnterBooleanValue(ctx *BooleanValueContext) {}

// ExitBooleanValue is called when production booleanValue is exited.
func (s *BaseEsqlBaseParserListener) ExitBooleanValue(ctx *BooleanValueContext) {}

// EnterNumericValue is called when production numericValue is entered.
func (s *BaseEsqlBaseParserListener) EnterNumericValue(ctx *NumericValueContext) {}

// ExitNumericValue is called when production numericValue is exited.
func (s *BaseEsqlBaseParserListener) ExitNumericValue(ctx *NumericValueContext) {}

// EnterDecimalValue is called when production decimalValue is entered.
func (s *BaseEsqlBaseParserListener) EnterDecimalValue(ctx *DecimalValueContext) {}

// ExitDecimalValue is called when production decimalValue is exited.
func (s *BaseEsqlBaseParserListener) ExitDecimalValue(ctx *DecimalValueContext) {}

// EnterIntegerValue is called when production integerValue is entered.
func (s *BaseEsqlBaseParserListener) EnterIntegerValue(ctx *IntegerValueContext) {}

// ExitIntegerValue is called when production integerValue is exited.
func (s *BaseEsqlBaseParserListener) ExitIntegerValue(ctx *IntegerValueContext) {}

// EnterString is called when production string is entered.
func (s *BaseEsqlBaseParserListener) EnterString(ctx *StringContext) {}

// ExitString is called when production string is exited.
func (s *BaseEsqlBaseParserListener) ExitString(ctx *StringContext) {}

// EnterComparisonOperator is called when production comparisonOperator is entered.
func (s *BaseEsqlBaseParserListener) EnterComparisonOperator(ctx *ComparisonOperatorContext) {}

// ExitComparisonOperator is called when production comparisonOperator is exited.
func (s *BaseEsqlBaseParserListener) ExitComparisonOperator(ctx *ComparisonOperatorContext) {}

// EnterExplainCommand is called when production explainCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterExplainCommand(ctx *ExplainCommandContext) {}

// ExitExplainCommand is called when production explainCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitExplainCommand(ctx *ExplainCommandContext) {}

// EnterSubqueryExpression is called when production subqueryExpression is entered.
func (s *BaseEsqlBaseParserListener) EnterSubqueryExpression(ctx *SubqueryExpressionContext) {}

// ExitSubqueryExpression is called when production subqueryExpression is exited.
func (s *BaseEsqlBaseParserListener) ExitSubqueryExpression(ctx *SubqueryExpressionContext) {}

// EnterShowInfo is called when production showInfo is entered.
func (s *BaseEsqlBaseParserListener) EnterShowInfo(ctx *ShowInfoContext) {}

// ExitShowInfo is called when production showInfo is exited.
func (s *BaseEsqlBaseParserListener) ExitShowInfo(ctx *ShowInfoContext) {}

// EnterEnrichCommand is called when production enrichCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterEnrichCommand(ctx *EnrichCommandContext) {}

// ExitEnrichCommand is called when production enrichCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitEnrichCommand(ctx *EnrichCommandContext) {}

// EnterEnrichPolicyName is called when production enrichPolicyName is entered.
func (s *BaseEsqlBaseParserListener) EnterEnrichPolicyName(ctx *EnrichPolicyNameContext) {}

// ExitEnrichPolicyName is called when production enrichPolicyName is exited.
func (s *BaseEsqlBaseParserListener) ExitEnrichPolicyName(ctx *EnrichPolicyNameContext) {}

// EnterEnrichWithClause is called when production enrichWithClause is entered.
func (s *BaseEsqlBaseParserListener) EnterEnrichWithClause(ctx *EnrichWithClauseContext) {}

// ExitEnrichWithClause is called when production enrichWithClause is exited.
func (s *BaseEsqlBaseParserListener) ExitEnrichWithClause(ctx *EnrichWithClauseContext) {}

// EnterChangePointCommand is called when production changePointCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterChangePointCommand(ctx *ChangePointCommandContext) {}

// ExitChangePointCommand is called when production changePointCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitChangePointCommand(ctx *ChangePointCommandContext) {}

// EnterSampleCommand is called when production sampleCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterSampleCommand(ctx *SampleCommandContext) {}

// ExitSampleCommand is called when production sampleCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitSampleCommand(ctx *SampleCommandContext) {}

// EnterLookupCommand is called when production lookupCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterLookupCommand(ctx *LookupCommandContext) {}

// ExitLookupCommand is called when production lookupCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitLookupCommand(ctx *LookupCommandContext) {}

// EnterInlinestatsCommand is called when production inlinestatsCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterInlinestatsCommand(ctx *InlinestatsCommandContext) {}

// ExitInlinestatsCommand is called when production inlinestatsCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitInlinestatsCommand(ctx *InlinestatsCommandContext) {}

// EnterJoinCommand is called when production joinCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterJoinCommand(ctx *JoinCommandContext) {}

// ExitJoinCommand is called when production joinCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitJoinCommand(ctx *JoinCommandContext) {}

// EnterJoinTarget is called when production joinTarget is entered.
func (s *BaseEsqlBaseParserListener) EnterJoinTarget(ctx *JoinTargetContext) {}

// ExitJoinTarget is called when production joinTarget is exited.
func (s *BaseEsqlBaseParserListener) ExitJoinTarget(ctx *JoinTargetContext) {}

// EnterJoinCondition is called when production joinCondition is entered.
func (s *BaseEsqlBaseParserListener) EnterJoinCondition(ctx *JoinConditionContext) {}

// ExitJoinCondition is called when production joinCondition is exited.
func (s *BaseEsqlBaseParserListener) ExitJoinCondition(ctx *JoinConditionContext) {}

// EnterJoinPredicate is called when production joinPredicate is entered.
func (s *BaseEsqlBaseParserListener) EnterJoinPredicate(ctx *JoinPredicateContext) {}

// ExitJoinPredicate is called when production joinPredicate is exited.
func (s *BaseEsqlBaseParserListener) ExitJoinPredicate(ctx *JoinPredicateContext) {}

// EnterInferenceCommandOptions is called when production inferenceCommandOptions is entered.
func (s *BaseEsqlBaseParserListener) EnterInferenceCommandOptions(ctx *InferenceCommandOptionsContext) {
}

// ExitInferenceCommandOptions is called when production inferenceCommandOptions is exited.
func (s *BaseEsqlBaseParserListener) ExitInferenceCommandOptions(ctx *InferenceCommandOptionsContext) {
}

// EnterInferenceCommandOption is called when production inferenceCommandOption is entered.
func (s *BaseEsqlBaseParserListener) EnterInferenceCommandOption(ctx *InferenceCommandOptionContext) {
}

// ExitInferenceCommandOption is called when production inferenceCommandOption is exited.
func (s *BaseEsqlBaseParserListener) ExitInferenceCommandOption(ctx *InferenceCommandOptionContext) {}

// EnterInferenceCommandOptionValue is called when production inferenceCommandOptionValue is entered.
func (s *BaseEsqlBaseParserListener) EnterInferenceCommandOptionValue(ctx *InferenceCommandOptionValueContext) {
}

// ExitInferenceCommandOptionValue is called when production inferenceCommandOptionValue is exited.
func (s *BaseEsqlBaseParserListener) ExitInferenceCommandOptionValue(ctx *InferenceCommandOptionValueContext) {
}

// EnterRerankCommand is called when production rerankCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterRerankCommand(ctx *RerankCommandContext) {}

// ExitRerankCommand is called when production rerankCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitRerankCommand(ctx *RerankCommandContext) {}

// EnterCompletionCommand is called when production completionCommand is entered.
func (s *BaseEsqlBaseParserListener) EnterCompletionCommand(ctx *CompletionCommandContext) {}

// ExitCompletionCommand is called when production completionCommand is exited.
func (s *BaseEsqlBaseParserListener) ExitCompletionCommand(ctx *CompletionCommandContext) {}
