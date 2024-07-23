package {{.PackageName}};

import lombok.RequiredArgsConstructor;
import org.springframework.beans.BeanUtils;
import org.springframework.web.bind.annotation.*;
import java.util.List;
{{- range .Imports}}
import {{.}};
{{- end}}

@RestController
@RequiredArgsConstructor
public class {{.ControllerName}} {

    private final {{.ServiceName}} {{.ServiceVariableName}};
    {{- $svn := .ServiceVariableName}}

{{- range .HttpRuleMap}}

    {{if .Method.HasComment}}// {{.Method.Comment}}{{- end}}
    @{{.HttpMethod}}Mapping(path = "{{.HttpPath}}")
    public {{.ResponseBody.Type}} {{.Method.Name}}(
    {{- $len := (len .Params) -}}
    {{- if gt (len .Params) 1 }}
        {{- range $index, $param := .Params }}
        {{- if gt $index 0 }},{{end}}
        {{- "\n\t\t"}}{{.Annotation | safe}} {{.Type}} {{.Name}}
        {{- end }}
    ) {
    {{- else }}
        {{- range $index, $param := .Params }}
        {{- if gt $index 0}}, {{end}}
        {{- .Annotation | safe}} {{.Type}} {{ .Name -}}
        {{- end }}) {
    {{- end }}

        {{ $rm := .RequestMessage -}}
        {{.RequestMessage.Type}} {{.RequestMessage.Name}} = new {{.RequestMessage.Type}}();
        {{- $rb := .RequestMessage -}}
        {{- range .PathParams}}
        {{$rb.Name}}.set{{.Name | ucfirst}}({{.Name}});
        {{- end}}

        {{- range .QueryParams}}
        {{$rm.Name}}.set{{.Name | ucfirst }}({{.Name}});
        {{- end}}

        {{- if .HasRequestBody}}
        {{- if not .IsWildcards }}
        {{.RequestMessage.Name}}.set{{.RequestBody.Name | ucfirst }}({{.RequestBody.Name}});
        {{- else }}
        BeanUtils.copyProperties({{.RequestBody.Name}}, {{.RequestMessage.Name}});
        {{- end}}
        {{- end}}

        return {{$svn}}.{{.Method.Name}}({{.RequestMessage.Name}});
    }
{{- end}}
}
