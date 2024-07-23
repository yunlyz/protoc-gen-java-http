package {{.PackageName}};

public enum {{.EnumName}} {
{{- range $index, $element := .Values}}
    {{- if $index}},{{end}}
    {{.Name}}({{.Code}}, "{{.Desc}}")
{{- end}};

    private final int code;
    private final String desc;

    {{.EnumName}}(int code, String desc) {
        this.code = code;
        this.desc = desc;
    }

    public int code() {
        return code;
    }

    public String value() {
        return value;
    }

    public static {{.EnumName}} fromCode(int code) {
        for ({{.EnumName}} status : {{.EnumName}}.values()) {
            if (status.getCode() == code) {
                return status;
            }
        }
        throw new IllegalArgumentException("No matching {{.EnumName}} for code " + code);
    }
}
