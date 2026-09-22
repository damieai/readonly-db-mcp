// Code generated from tools/es-language-parser/upstream/esql8/EsqlBaseParser.g4 by ANTLR 4.13.1. DO NOT EDIT.

package v8 // EsqlBaseParser
import "github.com/antlr4-go/antlr/v4"

// EsqlBaseParserListener is a complete listener for a parse tree produced by EsqlBaseParser.
type EsqlBaseParserListener interface {
	antlr.ParseTreeListener

	// EnterSingleStatement is called when entering the singleStatement production.
	EnterSingleStatement(c *SingleStatementContext)

	// EnterCompositeQuery is called when entering the compositeQuery production.
	EnterCompositeQuery(c *CompositeQueryContext)

	// EnterSingleCommandQuery is called when entering the singleCommandQuery production.
	EnterSingleCommandQuery(c *SingleCommandQueryContext)

	// EnterSourceCommand is called when entering the sourceCommand production.
	EnterSourceCommand(c *SourceCommandContext)

	// EnterProcessingCommand is called when entering the processingCommand production.
	EnterProcessingCommand(c *ProcessingCommandContext)

	// EnterWhereCommand is called when entering the whereCommand production.
	EnterWhereCommand(c *WhereCommandContext)

	// EnterMatchExpression is called when entering the matchExpression production.
	EnterMatchExpression(c *MatchExpressionContext)

	// EnterLogicalNot is called when entering the logicalNot production.
	EnterLogicalNot(c *LogicalNotContext)

	// EnterBooleanDefault is called when entering the booleanDefault production.
	EnterBooleanDefault(c *BooleanDefaultContext)

	// EnterIsNull is called when entering the isNull production.
	EnterIsNull(c *IsNullContext)

	// EnterRegexExpression is called when entering the regexExpression production.
	EnterRegexExpression(c *RegexExpressionContext)

	// EnterLogicalIn is called when entering the logicalIn production.
	EnterLogicalIn(c *LogicalInContext)

	// EnterLogicalBinary is called when entering the logicalBinary production.
	EnterLogicalBinary(c *LogicalBinaryContext)

	// EnterLikeExpression is called when entering the likeExpression production.
	EnterLikeExpression(c *LikeExpressionContext)

	// EnterRlikeExpression is called when entering the rlikeExpression production.
	EnterRlikeExpression(c *RlikeExpressionContext)

	// EnterLikeListExpression is called when entering the likeListExpression production.
	EnterLikeListExpression(c *LikeListExpressionContext)

	// EnterMatchBooleanExpression is called when entering the matchBooleanExpression production.
	EnterMatchBooleanExpression(c *MatchBooleanExpressionContext)

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

	// EnterDereference is called when entering the dereference production.
	EnterDereference(c *DereferenceContext)

	// EnterInlineCast is called when entering the inlineCast production.
	EnterInlineCast(c *InlineCastContext)

	// EnterConstantDefault is called when entering the constantDefault production.
	EnterConstantDefault(c *ConstantDefaultContext)

	// EnterParenthesizedExpression is called when entering the parenthesizedExpression production.
	EnterParenthesizedExpression(c *ParenthesizedExpressionContext)

	// EnterFunction is called when entering the function production.
	EnterFunction(c *FunctionContext)

	// EnterFunctionExpression is called when entering the functionExpression production.
	EnterFunctionExpression(c *FunctionExpressionContext)

	// EnterFunctionName is called when entering the functionName production.
	EnterFunctionName(c *FunctionNameContext)

	// EnterMapExpression is called when entering the mapExpression production.
	EnterMapExpression(c *MapExpressionContext)

	// EnterEntryExpression is called when entering the entryExpression production.
	EnterEntryExpression(c *EntryExpressionContext)

	// EnterToDataType is called when entering the toDataType production.
	EnterToDataType(c *ToDataTypeContext)

	// EnterRowCommand is called when entering the rowCommand production.
	EnterRowCommand(c *RowCommandContext)

	// EnterFields is called when entering the fields production.
	EnterFields(c *FieldsContext)

	// EnterField is called when entering the field production.
	EnterField(c *FieldContext)

	// EnterRerankFields is called when entering the rerankFields production.
	EnterRerankFields(c *RerankFieldsContext)

	// EnterRerankField is called when entering the rerankField production.
	EnterRerankField(c *RerankFieldContext)

	// EnterFromCommand is called when entering the fromCommand production.
	EnterFromCommand(c *FromCommandContext)

	// EnterIndexPattern is called when entering the indexPattern production.
	EnterIndexPattern(c *IndexPatternContext)

	// EnterClusterString is called when entering the clusterString production.
	EnterClusterString(c *ClusterStringContext)

	// EnterSelectorString is called when entering the selectorString production.
	EnterSelectorString(c *SelectorStringContext)

	// EnterUnquotedIndexString is called when entering the unquotedIndexString production.
	EnterUnquotedIndexString(c *UnquotedIndexStringContext)

	// EnterIndexString is called when entering the indexString production.
	EnterIndexString(c *IndexStringContext)

	// EnterMetadata is called when entering the metadata production.
	EnterMetadata(c *MetadataContext)

	// EnterMetadataOption is called when entering the metadataOption production.
	EnterMetadataOption(c *MetadataOptionContext)

	// EnterDeprecated_metadata is called when entering the deprecated_metadata production.
	EnterDeprecated_metadata(c *Deprecated_metadataContext)

	// EnterMetricsCommand is called when entering the metricsCommand production.
	EnterMetricsCommand(c *MetricsCommandContext)

	// EnterEvalCommand is called when entering the evalCommand production.
	EnterEvalCommand(c *EvalCommandContext)

	// EnterStatsCommand is called when entering the statsCommand production.
	EnterStatsCommand(c *StatsCommandContext)

	// EnterAggFields is called when entering the aggFields production.
	EnterAggFields(c *AggFieldsContext)

	// EnterAggField is called when entering the aggField production.
	EnterAggField(c *AggFieldContext)

	// EnterQualifiedName is called when entering the qualifiedName production.
	EnterQualifiedName(c *QualifiedNameContext)

	// EnterQualifiedNamePattern is called when entering the qualifiedNamePattern production.
	EnterQualifiedNamePattern(c *QualifiedNamePatternContext)

	// EnterQualifiedNamePatterns is called when entering the qualifiedNamePatterns production.
	EnterQualifiedNamePatterns(c *QualifiedNamePatternsContext)

	// EnterIdentifier is called when entering the identifier production.
	EnterIdentifier(c *IdentifierContext)

	// EnterIdentifierPattern is called when entering the identifierPattern production.
	EnterIdentifierPattern(c *IdentifierPatternContext)

	// EnterNullLiteral is called when entering the nullLiteral production.
	EnterNullLiteral(c *NullLiteralContext)

	// EnterQualifiedIntegerLiteral is called when entering the qualifiedIntegerLiteral production.
	EnterQualifiedIntegerLiteral(c *QualifiedIntegerLiteralContext)

	// EnterDecimalLiteral is called when entering the decimalLiteral production.
	EnterDecimalLiteral(c *DecimalLiteralContext)

	// EnterIntegerLiteral is called when entering the integerLiteral production.
	EnterIntegerLiteral(c *IntegerLiteralContext)

	// EnterBooleanLiteral is called when entering the booleanLiteral production.
	EnterBooleanLiteral(c *BooleanLiteralContext)

	// EnterInputParameter is called when entering the inputParameter production.
	EnterInputParameter(c *InputParameterContext)

	// EnterStringLiteral is called when entering the stringLiteral production.
	EnterStringLiteral(c *StringLiteralContext)

	// EnterNumericArrayLiteral is called when entering the numericArrayLiteral production.
	EnterNumericArrayLiteral(c *NumericArrayLiteralContext)

	// EnterBooleanArrayLiteral is called when entering the booleanArrayLiteral production.
	EnterBooleanArrayLiteral(c *BooleanArrayLiteralContext)

	// EnterStringArrayLiteral is called when entering the stringArrayLiteral production.
	EnterStringArrayLiteral(c *StringArrayLiteralContext)

	// EnterInputParam is called when entering the inputParam production.
	EnterInputParam(c *InputParamContext)

	// EnterInputNamedOrPositionalParam is called when entering the inputNamedOrPositionalParam production.
	EnterInputNamedOrPositionalParam(c *InputNamedOrPositionalParamContext)

	// EnterInputDoubleParams is called when entering the inputDoubleParams production.
	EnterInputDoubleParams(c *InputDoubleParamsContext)

	// EnterInputNamedOrPositionalDoubleParams is called when entering the inputNamedOrPositionalDoubleParams production.
	EnterInputNamedOrPositionalDoubleParams(c *InputNamedOrPositionalDoubleParamsContext)

	// EnterIdentifierOrParameter is called when entering the identifierOrParameter production.
	EnterIdentifierOrParameter(c *IdentifierOrParameterContext)

	// EnterLimitCommand is called when entering the limitCommand production.
	EnterLimitCommand(c *LimitCommandContext)

	// EnterSortCommand is called when entering the sortCommand production.
	EnterSortCommand(c *SortCommandContext)

	// EnterOrderExpression is called when entering the orderExpression production.
	EnterOrderExpression(c *OrderExpressionContext)

	// EnterKeepCommand is called when entering the keepCommand production.
	EnterKeepCommand(c *KeepCommandContext)

	// EnterDropCommand is called when entering the dropCommand production.
	EnterDropCommand(c *DropCommandContext)

	// EnterRenameCommand is called when entering the renameCommand production.
	EnterRenameCommand(c *RenameCommandContext)

	// EnterRenameClause is called when entering the renameClause production.
	EnterRenameClause(c *RenameClauseContext)

	// EnterDissectCommand is called when entering the dissectCommand production.
	EnterDissectCommand(c *DissectCommandContext)

	// EnterGrokCommand is called when entering the grokCommand production.
	EnterGrokCommand(c *GrokCommandContext)

	// EnterMvExpandCommand is called when entering the mvExpandCommand production.
	EnterMvExpandCommand(c *MvExpandCommandContext)

	// EnterCommandOptions is called when entering the commandOptions production.
	EnterCommandOptions(c *CommandOptionsContext)

	// EnterCommandOption is called when entering the commandOption production.
	EnterCommandOption(c *CommandOptionContext)

	// EnterBooleanValue is called when entering the booleanValue production.
	EnterBooleanValue(c *BooleanValueContext)

	// EnterNumericValue is called when entering the numericValue production.
	EnterNumericValue(c *NumericValueContext)

	// EnterDecimalValue is called when entering the decimalValue production.
	EnterDecimalValue(c *DecimalValueContext)

	// EnterIntegerValue is called when entering the integerValue production.
	EnterIntegerValue(c *IntegerValueContext)

	// EnterString is called when entering the string production.
	EnterString(c *StringContext)

	// EnterComparisonOperator is called when entering the comparisonOperator production.
	EnterComparisonOperator(c *ComparisonOperatorContext)

	// EnterExplainCommand is called when entering the explainCommand production.
	EnterExplainCommand(c *ExplainCommandContext)

	// EnterSubqueryExpression is called when entering the subqueryExpression production.
	EnterSubqueryExpression(c *SubqueryExpressionContext)

	// EnterShowInfo is called when entering the showInfo production.
	EnterShowInfo(c *ShowInfoContext)

	// EnterEnrichCommand is called when entering the enrichCommand production.
	EnterEnrichCommand(c *EnrichCommandContext)

	// EnterEnrichPolicyName is called when entering the enrichPolicyName production.
	EnterEnrichPolicyName(c *EnrichPolicyNameContext)

	// EnterEnrichWithClause is called when entering the enrichWithClause production.
	EnterEnrichWithClause(c *EnrichWithClauseContext)

	// EnterChangePointCommand is called when entering the changePointCommand production.
	EnterChangePointCommand(c *ChangePointCommandContext)

	// EnterSampleCommand is called when entering the sampleCommand production.
	EnterSampleCommand(c *SampleCommandContext)

	// EnterLookupCommand is called when entering the lookupCommand production.
	EnterLookupCommand(c *LookupCommandContext)

	// EnterInlinestatsCommand is called when entering the inlinestatsCommand production.
	EnterInlinestatsCommand(c *InlinestatsCommandContext)

	// EnterJoinCommand is called when entering the joinCommand production.
	EnterJoinCommand(c *JoinCommandContext)

	// EnterJoinTarget is called when entering the joinTarget production.
	EnterJoinTarget(c *JoinTargetContext)

	// EnterJoinCondition is called when entering the joinCondition production.
	EnterJoinCondition(c *JoinConditionContext)

	// EnterJoinPredicate is called when entering the joinPredicate production.
	EnterJoinPredicate(c *JoinPredicateContext)

	// EnterInferenceCommandOptions is called when entering the inferenceCommandOptions production.
	EnterInferenceCommandOptions(c *InferenceCommandOptionsContext)

	// EnterInferenceCommandOption is called when entering the inferenceCommandOption production.
	EnterInferenceCommandOption(c *InferenceCommandOptionContext)

	// EnterInferenceCommandOptionValue is called when entering the inferenceCommandOptionValue production.
	EnterInferenceCommandOptionValue(c *InferenceCommandOptionValueContext)

	// EnterRerankCommand is called when entering the rerankCommand production.
	EnterRerankCommand(c *RerankCommandContext)

	// EnterCompletionCommand is called when entering the completionCommand production.
	EnterCompletionCommand(c *CompletionCommandContext)

	// ExitSingleStatement is called when exiting the singleStatement production.
	ExitSingleStatement(c *SingleStatementContext)

	// ExitCompositeQuery is called when exiting the compositeQuery production.
	ExitCompositeQuery(c *CompositeQueryContext)

	// ExitSingleCommandQuery is called when exiting the singleCommandQuery production.
	ExitSingleCommandQuery(c *SingleCommandQueryContext)

	// ExitSourceCommand is called when exiting the sourceCommand production.
	ExitSourceCommand(c *SourceCommandContext)

	// ExitProcessingCommand is called when exiting the processingCommand production.
	ExitProcessingCommand(c *ProcessingCommandContext)

	// ExitWhereCommand is called when exiting the whereCommand production.
	ExitWhereCommand(c *WhereCommandContext)

	// ExitMatchExpression is called when exiting the matchExpression production.
	ExitMatchExpression(c *MatchExpressionContext)

	// ExitLogicalNot is called when exiting the logicalNot production.
	ExitLogicalNot(c *LogicalNotContext)

	// ExitBooleanDefault is called when exiting the booleanDefault production.
	ExitBooleanDefault(c *BooleanDefaultContext)

	// ExitIsNull is called when exiting the isNull production.
	ExitIsNull(c *IsNullContext)

	// ExitRegexExpression is called when exiting the regexExpression production.
	ExitRegexExpression(c *RegexExpressionContext)

	// ExitLogicalIn is called when exiting the logicalIn production.
	ExitLogicalIn(c *LogicalInContext)

	// ExitLogicalBinary is called when exiting the logicalBinary production.
	ExitLogicalBinary(c *LogicalBinaryContext)

	// ExitLikeExpression is called when exiting the likeExpression production.
	ExitLikeExpression(c *LikeExpressionContext)

	// ExitRlikeExpression is called when exiting the rlikeExpression production.
	ExitRlikeExpression(c *RlikeExpressionContext)

	// ExitLikeListExpression is called when exiting the likeListExpression production.
	ExitLikeListExpression(c *LikeListExpressionContext)

	// ExitMatchBooleanExpression is called when exiting the matchBooleanExpression production.
	ExitMatchBooleanExpression(c *MatchBooleanExpressionContext)

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

	// ExitDereference is called when exiting the dereference production.
	ExitDereference(c *DereferenceContext)

	// ExitInlineCast is called when exiting the inlineCast production.
	ExitInlineCast(c *InlineCastContext)

	// ExitConstantDefault is called when exiting the constantDefault production.
	ExitConstantDefault(c *ConstantDefaultContext)

	// ExitParenthesizedExpression is called when exiting the parenthesizedExpression production.
	ExitParenthesizedExpression(c *ParenthesizedExpressionContext)

	// ExitFunction is called when exiting the function production.
	ExitFunction(c *FunctionContext)

	// ExitFunctionExpression is called when exiting the functionExpression production.
	ExitFunctionExpression(c *FunctionExpressionContext)

	// ExitFunctionName is called when exiting the functionName production.
	ExitFunctionName(c *FunctionNameContext)

	// ExitMapExpression is called when exiting the mapExpression production.
	ExitMapExpression(c *MapExpressionContext)

	// ExitEntryExpression is called when exiting the entryExpression production.
	ExitEntryExpression(c *EntryExpressionContext)

	// ExitToDataType is called when exiting the toDataType production.
	ExitToDataType(c *ToDataTypeContext)

	// ExitRowCommand is called when exiting the rowCommand production.
	ExitRowCommand(c *RowCommandContext)

	// ExitFields is called when exiting the fields production.
	ExitFields(c *FieldsContext)

	// ExitField is called when exiting the field production.
	ExitField(c *FieldContext)

	// ExitRerankFields is called when exiting the rerankFields production.
	ExitRerankFields(c *RerankFieldsContext)

	// ExitRerankField is called when exiting the rerankField production.
	ExitRerankField(c *RerankFieldContext)

	// ExitFromCommand is called when exiting the fromCommand production.
	ExitFromCommand(c *FromCommandContext)

	// ExitIndexPattern is called when exiting the indexPattern production.
	ExitIndexPattern(c *IndexPatternContext)

	// ExitClusterString is called when exiting the clusterString production.
	ExitClusterString(c *ClusterStringContext)

	// ExitSelectorString is called when exiting the selectorString production.
	ExitSelectorString(c *SelectorStringContext)

	// ExitUnquotedIndexString is called when exiting the unquotedIndexString production.
	ExitUnquotedIndexString(c *UnquotedIndexStringContext)

	// ExitIndexString is called when exiting the indexString production.
	ExitIndexString(c *IndexStringContext)

	// ExitMetadata is called when exiting the metadata production.
	ExitMetadata(c *MetadataContext)

	// ExitMetadataOption is called when exiting the metadataOption production.
	ExitMetadataOption(c *MetadataOptionContext)

	// ExitDeprecated_metadata is called when exiting the deprecated_metadata production.
	ExitDeprecated_metadata(c *Deprecated_metadataContext)

	// ExitMetricsCommand is called when exiting the metricsCommand production.
	ExitMetricsCommand(c *MetricsCommandContext)

	// ExitEvalCommand is called when exiting the evalCommand production.
	ExitEvalCommand(c *EvalCommandContext)

	// ExitStatsCommand is called when exiting the statsCommand production.
	ExitStatsCommand(c *StatsCommandContext)

	// ExitAggFields is called when exiting the aggFields production.
	ExitAggFields(c *AggFieldsContext)

	// ExitAggField is called when exiting the aggField production.
	ExitAggField(c *AggFieldContext)

	// ExitQualifiedName is called when exiting the qualifiedName production.
	ExitQualifiedName(c *QualifiedNameContext)

	// ExitQualifiedNamePattern is called when exiting the qualifiedNamePattern production.
	ExitQualifiedNamePattern(c *QualifiedNamePatternContext)

	// ExitQualifiedNamePatterns is called when exiting the qualifiedNamePatterns production.
	ExitQualifiedNamePatterns(c *QualifiedNamePatternsContext)

	// ExitIdentifier is called when exiting the identifier production.
	ExitIdentifier(c *IdentifierContext)

	// ExitIdentifierPattern is called when exiting the identifierPattern production.
	ExitIdentifierPattern(c *IdentifierPatternContext)

	// ExitNullLiteral is called when exiting the nullLiteral production.
	ExitNullLiteral(c *NullLiteralContext)

	// ExitQualifiedIntegerLiteral is called when exiting the qualifiedIntegerLiteral production.
	ExitQualifiedIntegerLiteral(c *QualifiedIntegerLiteralContext)

	// ExitDecimalLiteral is called when exiting the decimalLiteral production.
	ExitDecimalLiteral(c *DecimalLiteralContext)

	// ExitIntegerLiteral is called when exiting the integerLiteral production.
	ExitIntegerLiteral(c *IntegerLiteralContext)

	// ExitBooleanLiteral is called when exiting the booleanLiteral production.
	ExitBooleanLiteral(c *BooleanLiteralContext)

	// ExitInputParameter is called when exiting the inputParameter production.
	ExitInputParameter(c *InputParameterContext)

	// ExitStringLiteral is called when exiting the stringLiteral production.
	ExitStringLiteral(c *StringLiteralContext)

	// ExitNumericArrayLiteral is called when exiting the numericArrayLiteral production.
	ExitNumericArrayLiteral(c *NumericArrayLiteralContext)

	// ExitBooleanArrayLiteral is called when exiting the booleanArrayLiteral production.
	ExitBooleanArrayLiteral(c *BooleanArrayLiteralContext)

	// ExitStringArrayLiteral is called when exiting the stringArrayLiteral production.
	ExitStringArrayLiteral(c *StringArrayLiteralContext)

	// ExitInputParam is called when exiting the inputParam production.
	ExitInputParam(c *InputParamContext)

	// ExitInputNamedOrPositionalParam is called when exiting the inputNamedOrPositionalParam production.
	ExitInputNamedOrPositionalParam(c *InputNamedOrPositionalParamContext)

	// ExitInputDoubleParams is called when exiting the inputDoubleParams production.
	ExitInputDoubleParams(c *InputDoubleParamsContext)

	// ExitInputNamedOrPositionalDoubleParams is called when exiting the inputNamedOrPositionalDoubleParams production.
	ExitInputNamedOrPositionalDoubleParams(c *InputNamedOrPositionalDoubleParamsContext)

	// ExitIdentifierOrParameter is called when exiting the identifierOrParameter production.
	ExitIdentifierOrParameter(c *IdentifierOrParameterContext)

	// ExitLimitCommand is called when exiting the limitCommand production.
	ExitLimitCommand(c *LimitCommandContext)

	// ExitSortCommand is called when exiting the sortCommand production.
	ExitSortCommand(c *SortCommandContext)

	// ExitOrderExpression is called when exiting the orderExpression production.
	ExitOrderExpression(c *OrderExpressionContext)

	// ExitKeepCommand is called when exiting the keepCommand production.
	ExitKeepCommand(c *KeepCommandContext)

	// ExitDropCommand is called when exiting the dropCommand production.
	ExitDropCommand(c *DropCommandContext)

	// ExitRenameCommand is called when exiting the renameCommand production.
	ExitRenameCommand(c *RenameCommandContext)

	// ExitRenameClause is called when exiting the renameClause production.
	ExitRenameClause(c *RenameClauseContext)

	// ExitDissectCommand is called when exiting the dissectCommand production.
	ExitDissectCommand(c *DissectCommandContext)

	// ExitGrokCommand is called when exiting the grokCommand production.
	ExitGrokCommand(c *GrokCommandContext)

	// ExitMvExpandCommand is called when exiting the mvExpandCommand production.
	ExitMvExpandCommand(c *MvExpandCommandContext)

	// ExitCommandOptions is called when exiting the commandOptions production.
	ExitCommandOptions(c *CommandOptionsContext)

	// ExitCommandOption is called when exiting the commandOption production.
	ExitCommandOption(c *CommandOptionContext)

	// ExitBooleanValue is called when exiting the booleanValue production.
	ExitBooleanValue(c *BooleanValueContext)

	// ExitNumericValue is called when exiting the numericValue production.
	ExitNumericValue(c *NumericValueContext)

	// ExitDecimalValue is called when exiting the decimalValue production.
	ExitDecimalValue(c *DecimalValueContext)

	// ExitIntegerValue is called when exiting the integerValue production.
	ExitIntegerValue(c *IntegerValueContext)

	// ExitString is called when exiting the string production.
	ExitString(c *StringContext)

	// ExitComparisonOperator is called when exiting the comparisonOperator production.
	ExitComparisonOperator(c *ComparisonOperatorContext)

	// ExitExplainCommand is called when exiting the explainCommand production.
	ExitExplainCommand(c *ExplainCommandContext)

	// ExitSubqueryExpression is called when exiting the subqueryExpression production.
	ExitSubqueryExpression(c *SubqueryExpressionContext)

	// ExitShowInfo is called when exiting the showInfo production.
	ExitShowInfo(c *ShowInfoContext)

	// ExitEnrichCommand is called when exiting the enrichCommand production.
	ExitEnrichCommand(c *EnrichCommandContext)

	// ExitEnrichPolicyName is called when exiting the enrichPolicyName production.
	ExitEnrichPolicyName(c *EnrichPolicyNameContext)

	// ExitEnrichWithClause is called when exiting the enrichWithClause production.
	ExitEnrichWithClause(c *EnrichWithClauseContext)

	// ExitChangePointCommand is called when exiting the changePointCommand production.
	ExitChangePointCommand(c *ChangePointCommandContext)

	// ExitSampleCommand is called when exiting the sampleCommand production.
	ExitSampleCommand(c *SampleCommandContext)

	// ExitLookupCommand is called when exiting the lookupCommand production.
	ExitLookupCommand(c *LookupCommandContext)

	// ExitInlinestatsCommand is called when exiting the inlinestatsCommand production.
	ExitInlinestatsCommand(c *InlinestatsCommandContext)

	// ExitJoinCommand is called when exiting the joinCommand production.
	ExitJoinCommand(c *JoinCommandContext)

	// ExitJoinTarget is called when exiting the joinTarget production.
	ExitJoinTarget(c *JoinTargetContext)

	// ExitJoinCondition is called when exiting the joinCondition production.
	ExitJoinCondition(c *JoinConditionContext)

	// ExitJoinPredicate is called when exiting the joinPredicate production.
	ExitJoinPredicate(c *JoinPredicateContext)

	// ExitInferenceCommandOptions is called when exiting the inferenceCommandOptions production.
	ExitInferenceCommandOptions(c *InferenceCommandOptionsContext)

	// ExitInferenceCommandOption is called when exiting the inferenceCommandOption production.
	ExitInferenceCommandOption(c *InferenceCommandOptionContext)

	// ExitInferenceCommandOptionValue is called when exiting the inferenceCommandOptionValue production.
	ExitInferenceCommandOptionValue(c *InferenceCommandOptionValueContext)

	// ExitRerankCommand is called when exiting the rerankCommand production.
	ExitRerankCommand(c *RerankCommandContext)

	// ExitCompletionCommand is called when exiting the completionCommand production.
	ExitCompletionCommand(c *CompletionCommandContext)
}
