package {{.PackageName}};

import jakarta.validation.constraints.NotBlank;
import java.io.Serializable;
import lombok.Data;
{{- range .Imports}}
import {{ . }};
{{- end}}

@Data
public class {{ .BeanName }} implements Serializable {
{{- range .Fields }}
    {{- "\n" }}
    {{- if .HasComment }}{{ "\n\t" }}// {{ .Comment }}{{- end }}
    {{- if .IsRequired }}{{ "\n\t" }}@NotBlank(message = "{{ .I18n }}不能为空"){{- end }}
    {{ .FieldType | safe }} {{ .FieldName }};
{{- end }}

{{- range .Oneofs }}
    {{ $oen := .OneofEnumName }}{{ $ofn := .OneofFieldName }}
    public enum {{ .OneofEnumName }} {
        {{ .OneofNotSet }}
        {{- range .OneofCases }}
        {{ . }},
        {{- end }}
    }

    private {{ .OneofEnumName }} {{ $ofn }} = {{ .OneofEnumName }}.{{ .OneofNotSet }};

    public {{ .OneofEnumName }} get{{ .OneofEnumName }}() {
        return {{ .OneofFieldName }};
    }

    {{- $ofs := .OneofFields -}}
    {{- range .OneofFields }}
    {{ $fn := .FieldName }}{{ $FN := (ucfirst .FieldName) }}{{ $EN := .EnumName }}
    public void set{{ $FN }}({{ .FieldType | safe }} {{ .FieldName }}) {
        this.{{ .FieldName }} = {{ .FieldName }};

        {{- range $ofs }}
        {{- if ne .FieldName $fn }}
        this.{{ .FieldName }} = null;
        {{- end }}
        {{- end }}
        this.{{ $ofn }} = {{ $oen }}.{{ $EN }};
    }

    public {{ .FieldType | safe }} get{{ $FN }}() {
        if (this.{{ $fn }} == {{ $oen }}.{{ $FN }}) {
            return this.{{ .FieldName }};
        }
        return null;
    }

    public boolean has{{ $FN }}() {
        return this.{{ $fn }} == {{ $oen }}.{{ $FN }};
    }
    {{- end }}
{{- end }}
}
