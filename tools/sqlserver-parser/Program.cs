using System.Buffers.Binary;
using System.Collections;
using System.Reflection;
using System.Text.Json;
using Microsoft.SqlServer.TransactSql.ScriptDom;

// One bounded request per process: no sockets, credentials, files or caller
// selected code. The parent admission permit bounds concurrent helper processes.
const int Maximum = 1 << 20;
var input = Console.OpenStandardInput();
var output = Console.OpenStandardOutput();
try
{
    byte[] header = new byte[4];
    input.ReadExactly(header);
    int length = BinaryPrimitives.ReadInt32BigEndian(header);
    if (length < 1 || length > Maximum) throw new InvalidDataException();
    byte[] body = new byte[length]; input.ReadExactly(body);
    var request = JsonSerializer.Deserialize<Request>(body) ?? throw new InvalidDataException();
    TSqlParser parser = request.Compatibility switch {
        150 => new TSql150Parser(true), 160 => new TSql160Parser(true),
        170 => new TSql170Parser(true), _ => throw new InvalidDataException()
    };
    var fragment = parser.Parse(new StringReader(request.SQL), out var errors);
    if (errors.Count != 0) throw new InvalidDataException("syntax");
    var analysis = new Analysis(request.Module);
    analysis.Inspect(fragment);
    Reply(new { protocol = 1, accepted = true, references = analysis.References,
        parameters = analysis.Parameters.Order().ToArray(), nodes = analysis.Nodes });
}
catch
{
    // Parser diagnostics can contain caller SQL. The protocol returns only a
    // stable classification and never writes source text to stderr.
    Reply(new { protocol = 1, accepted = false, error = "unproven_tsql" });
}
void Reply(object value)
{
    byte[] bytes = JsonSerializer.SerializeToUtf8Bytes(value);
    if (bytes.Length > Maximum) bytes = JsonSerializer.SerializeToUtf8Bytes(new { protocol = 1, accepted = false });
    byte[] header = new byte[4]; BinaryPrimitives.WriteInt32BigEndian(header, bytes.Length);
    output.Write(header); output.Write(bytes); output.Flush();
}
record Request(string SQL, int Compatibility, bool Module);
record Reference(string[] parts, string kind);

sealed class Analysis(bool module)
{
    public List<Reference> References { get; } = [];
    public HashSet<string> Parameters { get; } = new(StringComparer.OrdinalIgnoreCase);
    public int Nodes { get; private set; }
    static readonly HashSet<string> External = new(StringComparer.OrdinalIgnoreCase) {
        "OpenRowsetTableReference", "OpenQueryTableReference", "AdHocTableReference",
        "OpenXmlTableReference", "NextValueForExpression", "SelectSetVariable",
        "ExecuteSpecification", "ExecuteStatement", "ExecuteAsClause", "OutputIntoClause" };
    static readonly HashSet<string> ReadTables = new(StringComparer.Ordinal) {
        "NamedTableReference", "SchemaObjectFunctionTableReference", "FullTextTableReference",
        "SemanticTableReference", "OpenJsonTableReference", "BuiltInFunctionTableReference",
        "GlobalFunctionTableReference", "PivotedTableReference", "UnpivotedTableReference",
        "JoinParenthesisTableReference", "OdbcQualifiedJoinTableReference", "QueryDerivedTable",
        "InlineDerivedTable", "QualifiedJoin", "UnqualifiedJoin", "PredictTableReference",
        "ChangeTableChangesTableReference", "ChangeTableVersionTableReference",
        "VariableTableReference", "VariableMethodCallTableReference", "VectorSearchTableReference",
        "AIGenerateFixedChunksTableReference"
    };
    public void Inspect(TSqlFragment root)
    {
        if (root is not TSqlScript script || script.Batches.Count != 1 || script.Batches[0].Statements.Count != 1)
            throw new InvalidDataException();
        var statement = script.Batches[0].Statements[0];
        if (!module && statement is not SelectStatement) throw new InvalidDataException();
        if (module && statement.GetType().Name is not ("CreateFunctionStatement" or "AlterFunctionStatement" or "CreateOrAlterFunctionStatement" or "CreateViewStatement" or "AlterViewStatement" or "CreateOrAlterViewStatement"))
            throw new InvalidDataException();
        Walk(root, new(StringComparer.OrdinalIgnoreCase), 0, statement);
    }
    void Walk(TSqlFragment node, HashSet<string> ctes, int depth, TSqlStatement rootStatement)
    {
        if (++Nodes > 100000 || depth > 256) throw new InvalidDataException();
        string type = node.GetType().Name;
        if (node is TableReference && !ReadTables.Contains(type)) throw new InvalidDataException();
        if (External.Contains(type) && !(module && type == "SelectSetVariable")) throw new InvalidDataException();
        if (node is TSqlStatement statement && statement != rootStatement && statement is not SelectStatement)
        {
            // Local scalar calculation in T-SQL functions cannot persist data.
            bool localDml = module && node is DataModificationStatement dml &&
                dml.GetType().GetProperty(type.Replace("Statement", "Specification"))?.GetValue(dml) is DataModificationSpecification specification &&
                specification.Target is VariableTableReference;
            if (!localDml && (!module || type is not ("ReturnStatement" or "BeginEndBlockStatement" or "DeclareVariableStatement" or "DeclareTableVariableStatement" or "SetVariableStatement" or "IfStatement" or "WhileStatement" or "BreakStatement" or "ContinueStatement")))
                throw new InvalidDataException();
        }
        if (node is SelectStatement select)
        {
            if (select.Into != null) throw new InvalidDataException();
            if (select.WithCtesAndXmlNamespaces != null)
            {
                ctes = new(ctes, StringComparer.OrdinalIgnoreCase);
                foreach (var cte in select.WithCtesAndXmlNamespaces.CommonTableExpressions) ctes.Add(cte.ExpressionName.Value);
            }
        }
        if (node is NamedTableReference table)
        {
            var parts = table.SchemaObject.Identifiers.Select(x => x.Value).ToArray();
            if (!(parts.Length == 1 && ctes.Contains(parts[0]))) References.Add(new(parts, "relation"));
        }
        if (node is SchemaObjectFunctionTableReference tvf)
            References.Add(new(tvf.SchemaObject.Identifiers.Select(x => x.Value).ToArray(), "function"));
        if (node is FullTextTableReference fullText)
            References.Add(new(fullText.TableName.Identifiers.Select(x => x.Value).ToArray(), "relation"));
        if (node is SemanticTableReference semantic)
            References.Add(new(semantic.TableName.Identifiers.Select(x => x.Value).ToArray(), "relation"));
        if (type == "VectorSearchTableReference" && node.GetType().GetProperty("Table")?.GetValue(node) is SchemaObjectName vectorTable)
            References.Add(new(vectorTable.Identifiers.Select(x => x.Value).ToArray(), "relation"));
        if (node is ChangeTableChangesTableReference changes)
            References.Add(new(changes.Target.Identifiers.Select(x => x.Value).ToArray(), "relation"));
        if (node is ChangeTableVersionTableReference version)
            References.Add(new(version.Target.Identifiers.Select(x => x.Value).ToArray(), "relation"));
        if (node is FunctionCall call)
        {
            if (call.CallTarget is MultiPartIdentifierCallTarget target)
                References.Add(new(target.MultiPartIdentifier.Identifiers.Select(x => x.Value).Append(call.FunctionName.Value).ToArray(), "function"));
            if (call.FunctionName.Value.StartsWith("AI_GENERATE_", StringComparison.OrdinalIgnoreCase) || call.FunctionName.Value.StartsWith("OPENROWSET", StringComparison.OrdinalIgnoreCase))
                throw new InvalidDataException();
        }
        if (!module && node is VariableReference variable) Parameters.Add(variable.Name);
        // Reflect every child property in the pinned AST assembly. New nested
        // nodes cannot disappear merely because a hand-written visitor missed a
        // branch; statement/effect checks run at every depth.
        foreach (var property in node.GetType().GetProperties(BindingFlags.Instance | BindingFlags.Public))
        {
            if (property.GetIndexParameters().Length != 0 || property.Name == "ScriptTokenStream") continue;
            var value = property.GetValue(node);
            if (value is TSqlFragment child) Walk(child, ctes, depth + 1, rootStatement);
            else if (value is IEnumerable sequence && value is not string)
                foreach (var item in sequence) if (item is TSqlFragment fragment) Walk(fragment, ctes, depth + 1, rootStatement);
        }
    }
}
