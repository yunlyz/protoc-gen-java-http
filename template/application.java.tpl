package {{.PackageName}};
{{ print "" }}
{{- range .Imports }}
import {{ . }};
{{- end}}

public interface {{.ServiceName}} {
{{- range .Methods}}

    // {{.Comment}}
    {{.Response}} {{.Name}}({{.Request}} request);
{{- end}}
}