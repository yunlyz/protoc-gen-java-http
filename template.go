package main

import (
	"bytes"
	"fmt"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"html/template"
	"net/http"
	"regexp"
	"strings"
)

var funcMap = template.FuncMap{
	"safe":    func(s string) template.HTML { return template.HTML(s) },
	"mod":     func(a, b int) int { return a % b },
	"add":     func(a, b int) int { return a + b },
	"ucfirst": ucfirst,
	"lcfirst": lcfirst,
}

type Template interface {
	FileName() string
	Execute() string
}

type ApplicationTemplate struct {
	PackageName string                       // Java package name
	ServiceType string                       // Greeter
	ServiceName string                       // helloworld.Greeter
	Methods     []*MethodDescriptor          // [Hello, World]
	MethodMap   map[string]*MethodDescriptor // Map
	Messages    map[string]*protogen.Message
	Imports     []string
	Comments    string
}

func NewApplicationJavaTemplate(service *protogen.Service) Template {
	methods := make([]*MethodDescriptor, 0)
	methodMap := make(map[string]*MethodDescriptor)
	imports := make([]string, 0)
	for _, method := range service.Methods {
		md := NewMethodDescriptor(method)
		methods = append(methods, md)
		methodMap[method.GoName] = md
		imports = append(imports, BeanImportPath(method.Input), BeanImportPath(method.Output))
	}

	return &ApplicationTemplate{
		PackageName: ApplicationPackageName(service),
		ServiceType: service.GoName,
		ServiceName: ApplicationServiceName(service),
		Methods:     methods,
		MethodMap:   methodMap,
		Imports:     RemoveDuplicates(imports),
		Comments:    TrimComments(service.Comments.Leading.String()),
	}
}

func (t *ApplicationTemplate) FileName() string {
	return Package2Path(t.PackageName) + "/" + t.ServiceName + ".java"
}

func (t *ApplicationTemplate) Execute() string {
	for _, method := range t.Methods {
		t.MethodMap[method.Name] = method
	}

	tmpl, err := template.New("http").Parse(applicationTemplate)
	if err != nil {
		panic(err)
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, t); err != nil {
		panic(err)
	}

	return strings.TrimSpace(buf.String())
}

type MethodDescriptor struct {
	method *protogen.Method

	// proto base info
	Name         string
	OriginalName string
	Request      string
	Response     string
	Comment      string
	HasComment   bool
	I18n         string

	// http rule
	HttpRule HttpRuleDescriptor
}

func NewMethodDescriptor(method *protogen.Method) *MethodDescriptor {
	comments := TrimComments(method.Comments.Leading.String())
	return &MethodDescriptor{
		method:       method,
		Name:         lcfirst(method.GoName),
		OriginalName: string(method.Desc.FullName()),
		Request:      method.Input.GoIdent.GoName,
		Response:     method.Output.GoIdent.GoName,
		Comment:      comments,
		HasComment:   comments != "",
		I18n:         method.GoName,
	}
}

type MessageTemplate struct {
	message *protogen.Message
	spring  *SpringBootPlugin

	PackageName string
	BeanName    string
	Comment     string
	Fields      []*FieldDescriptor
	Imports     []string
	Oneofs      []*OneofDescriptor
}

func (bean *MessageTemplate) FileName() string {
	return Package2Path(bean.PackageName+"."+bean.BeanName) + ".java"
}

func (bean *MessageTemplate) Execute() string {
	tmpl, err := template.New("bean").Funcs(funcMap).Parse(beanTemplate)
	if err != nil {
		panic(err)
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, bean); err != nil {
		panic(err)
	}

	return strings.TrimSpace(buf.String())

}

func NewMessageTemplate(spring *SpringBootPlugin, message *protogen.Message) Template {
	fields := make([]*FieldDescriptor, 0)
	imports := make([]string, 0)
	for _, field := range message.Fields {
		fields = append(fields, NewFieldDescriptor(field))
		if path, ok := FieldImportPath(field); ok {
			imports = append(imports, path)
		}
	}

	oneofs := make([]*OneofDescriptor, 0)
	for _, oneof := range message.Oneofs {
		oneofs = append(oneofs, NewOneofDescriptor(oneof))
	}

	return &MessageTemplate{
		message:     message,
		spring:      spring,
		PackageName: BeanPackageName(message),
		BeanName:    message.GoIdent.GoName,
		Comment:     TrimComments(message.Comments.Leading.String()),
		Fields:      fields,
		Oneofs:      oneofs,
		Imports:     RemoveDuplicates(imports),
	}
}

type OneofDescriptor struct {
	OneofFieldName string
	OneofEnumName  string
	OneofCases     []string
	OneofFields    []*FieldDescriptor
	OneofNotSet    string
}

func NewOneofDescriptor(oneof *protogen.Oneof) *OneofDescriptor {
	cases := make([]string, 0)
	fields := make([]*FieldDescriptor, 0)
	for _, field := range oneof.Fields {
		cases = append(cases, strings.ToUpper(field.Desc.TextName()))
		fields = append(fields, NewFieldDescriptor(field))
	}

	return &OneofDescriptor{
		OneofFieldName: lcfirst(oneof.GoName),
		OneofEnumName:  "Oneof" + oneof.GoName,
		OneofCases:     cases,
		OneofFields:    fields,
		OneofNotSet:    "UNSPECIFIED",
	}
}

type FieldDescriptor struct {
	field      *protogen.Field
	FieldName  string
	FieldType  string
	Comment    string
	HasComment bool
	Required   string
	IsRequired bool
	I18n       string
	Imports    string
	EnumName   string
}

func (f *FieldDescriptor) IsObject() bool {
	switch f.field.Desc.Kind() {
	case protoreflect.EnumKind | protoreflect.MessageKind:
		return true
	default:
		return false
	}
}

func NewFieldDescriptor(field *protogen.Field) *FieldDescriptor {
	isRequired := false
	behaviors := proto.GetExtension(field.Desc.Options(), annotations.E_FieldBehavior).([]annotations.FieldBehavior)
	for _, behavior := range behaviors {
		switch behavior {
		case annotations.FieldBehavior_REQUIRED:
			isRequired = true
		default:
			isRequired = false
		}
	}

	comments := TrimComments(field.Comments.Leading.String())
	i18n := ""
	if len(comments) > 0 {
		i18n = comments
	} else {
		i18n = field.GoName
	}

	return &FieldDescriptor{
		field:      field,
		FieldName:  lcfirst(field.GoName),
		FieldType:  NewJavaKind(field).JavaString(),
		Comment:    comments,
		HasComment: comments != "",
		IsRequired: isRequired,
		I18n:       i18n,
		EnumName:   strings.ToUpper(field.Desc.TextName()),
	}
}

type EnumValue struct {
	Name string
	Code int
	Desc string
}

type EnumTemplate struct {
	enum *protogen.Enum

	PackageName string
	EnumName    string
	Comment     string
	Values      []EnumValue
}

func (enum *EnumTemplate) FileName() string {
	return Package2Path(enum.PackageName+"."+enum.EnumName) + ".java"
}

func (enum *EnumTemplate) Execute() string {
	tmpl, err := template.New("enum").Parse(enumTemplate)
	if err != nil {
		panic(err)
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, enum); err != nil {
		panic(err)
	}

	return strings.TrimSpace(buf.String())
}

func NewEnumDescriptor(enum *protogen.Enum) Template {
	values := make([]EnumValue, 0)

	for _, value := range enum.Values {
		desc := TrimComments(value.Comments.Leading.String())
		if desc == "" {
			desc = string(value.Desc.Name())
		}
		values = append(values, EnumValue{
			Name: string(value.Desc.Name()),
			Code: value.Desc.Index(),
			Desc: desc,
		})
	}

	return &EnumTemplate{
		enum:        enum,
		PackageName: EnumPackageName(enum),
		EnumName:    string(enum.Desc.Name()),
		Comment:     TrimComments(enum.Comments.Leading.String()),
		Values:      values,
	}
}

type ControllerTemplate struct {
	service *protogen.Service

	PackageName         string
	ControllerName      string
	Methods             []*MethodDescriptor
	Imports             []string
	HttpRuleMap         map[string]*HttpRuleDescriptor
	ServiceName         string
	ServiceVariableName string
}

func (ctl *ControllerTemplate) FileName() string {
	return Package2Path(ctl.PackageName+"."+ctl.ControllerName) + ".java"
}

func (ctl *ControllerTemplate) Execute() string {
	tmpl, err := template.New("ctl").Funcs(funcMap).Parse(controllerTemplate)
	if err != nil {
		panic(err)
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, ctl); err != nil {
		panic(err)
	}

	return strings.TrimSpace(buf.String())
}

func NewControllerTemplate(service *protogen.Service) Template {
	methods := make([]*MethodDescriptor, 0)
	imports := []string{
		ApplicationImportPath(service),
	}
	httpRuleMap := make(map[string]*HttpRuleDescriptor)

	for _, method := range service.Methods {
		methods = append(methods, NewMethodDescriptor(method))
		imports = append(imports, BeanImportPaths(method.Input)...)
		imports = append(imports, BeanImportPaths(method.Output)...)
		httpRuleMap[method.GoName] = NewHttpRuleDescriptor(method)
	}
	serviceName := ApplicationServiceName(service)

	return &ControllerTemplate{
		service:             service,
		PackageName:         ControllerPackageName(service),
		ControllerName:      ControllerName(service),
		Methods:             methods,
		Imports:             RemoveDuplicates(imports),
		HttpRuleMap:         httpRuleMap,
		ServiceName:         serviceName,
		ServiceVariableName: lcfirst(serviceName),
	}
}

type JavaKindDescriptor struct {
	field *protogen.Field
}

func (kind *JavaKindDescriptor) JavaString() string {
	if kind.field.Desc.IsMap() {
		key := kind.field.Desc.MapKey()
		value := kind.field.Desc.MapValue()
		return fmt.Sprintf("Map<%s, %s>", LowLevelJavaKind(key), LowLevelJavaKind(value))
	} else if kind.field.Desc.IsList() {
		return fmt.Sprintf("List<%s>", HighLevelJavaKind(kind.field))
	} else {
		return HighLevelJavaKind(kind.field)
	}
}

func NewJavaKind(field *protogen.Field) *JavaKindDescriptor {
	return &JavaKindDescriptor{field: field}
}

func HighLevelJavaKind(field *protogen.Field) string {
	return LowLevelJavaKind(field.Desc)
}

func LowLevelJavaKind(field protoreflect.FieldDescriptor) string {
	switch field.Kind() {
	case protoreflect.Int32Kind:
		fallthrough
	case protoreflect.Sint32Kind:
		fallthrough
	case protoreflect.Uint32Kind:
		fallthrough
	case protoreflect.Sfixed32Kind:
		fallthrough
	case protoreflect.Fixed32Kind:
		return "Integer"
	case protoreflect.Int64Kind:
		fallthrough
	case protoreflect.Sint64Kind:
		fallthrough
	case protoreflect.Uint64Kind:
		fallthrough
	case protoreflect.Sfixed64Kind:
		fallthrough
	case protoreflect.Fixed64Kind:
		return "Long"
	case protoreflect.FloatKind:
		return "Float"
	case protoreflect.DoubleKind:
		return "Double"
	case protoreflect.BoolKind:
		return "Boolean"
	case protoreflect.EnumKind:
		return string(field.Enum().Name())
	case protoreflect.StringKind:
		return "String"
	case protoreflect.BytesKind:
		return "ByteString"
	case protoreflect.MessageKind:
		return string(field.Message().Name())
	case protoreflect.GroupKind:
		return "Object"
	default:
		return fmt.Sprintf("Kind(%s)", field.Kind())
	}
}

type HttpRuleDescriptor struct {
	// google/api/http.proto
	Method         *MethodDescriptor
	HttpMethod     string    // http method. GET/POST/PUT/DELETE
	HttpPath       string    // http path. Such as: /v1/helloworld/greeter
	RequestBody    Parameter // http request body
	ResponseBody   Parameter // http response body
	PathParams     []Parameter
	QueryParams    []Parameter
	HasPathParams  bool
	NeedNewLine    bool
	HasRequestBody bool
	HasQueryParams bool
	RequestMessage Parameter

	Params      []Parameter
	IsWildcards bool
}

func NewHttpRuleDescriptor(method *protogen.Method) *HttpRuleDescriptor {
	httpRule := proto.GetExtension(method.Desc.Options(), annotations.E_Http).(*annotations.HttpRule)

	httpMethod := BuildHttpMethod(httpRule)
	httpPath := BuildHttpPath(httpRule)

	pathVars := BuildPathVars(httpPath)
	pathParams := BuildPathParams(pathVars, method.Input)
	queryParams := BuildQueryParameters(method.Input, httpRule, pathVars)
	requestBody := BuildRequestBody(method.Input, httpRule)

	// 如果 [HttpRule.body][google.api.HttpRule.body] 为“*”，则没有 URL 查询参数，所有字段都通过 URL 路径和 HTTP 请求正文传递。
	// 如果 [HttpRule.body][google.api.HttpRule.body] 省略，则没有 HTTP 请求正文，所有字段都通过 URL 路径和 URL 查询参数传递。
	// The name of the request field whose value is mapped to the HTTP request body,
	// or * for mapping all request fields not captured by the path pattern to the HTTP body,
	// or omitted for not having any HTTP request body.
	hasRequestBody := HasRequestBody(httpRule)
	// query := Parameter{Type: method.Input.GoIdent.GoName, Name: ucfirst(method.Input.GoIdent.GoName)}
	params := make([]Parameter, 0)
	params = append(params, pathParams...)
	params = append(params, queryParams...)
	if hasRequestBody {
		params = append(params, requestBody)
	}

	return &HttpRuleDescriptor{
		Method:         NewMethodDescriptor(method),
		HttpMethod:     ucfirst(strings.ToLower(httpMethod)),
		HttpPath:       httpPath,
		RequestMessage: Parameter{Type: method.Input.GoIdent.GoName, Name: lcfirst(method.Input.GoIdent.GoName)},
		RequestBody:    requestBody,
		ResponseBody:   BuildResponseBody(method.Output, httpRule),
		PathParams:     pathParams,
		QueryParams:    queryParams,
		HasPathParams:  len(pathParams) > 0,
		HasQueryParams: HasQueryParams(httpRule),
		HasRequestBody: hasRequestBody,
		IsWildcards:    httpRule.Body == "*",
		Params:         params,
	}
}

func HasQueryParams(httpRule *annotations.HttpRule) bool {
	return httpRule.Body != "*"
}

func HasRequestBody(httpRule *annotations.HttpRule) bool {
	return httpRule.Body != ""
}

func BuildHttpMethod(httpRule *annotations.HttpRule) string {
	switch pattern := httpRule.Pattern.(type) {
	case *annotations.HttpRule_Get:
		return http.MethodGet
	case *annotations.HttpRule_Put:
		return http.MethodPut
	case *annotations.HttpRule_Post:
		return http.MethodPost
	case *annotations.HttpRule_Delete:
		return http.MethodDelete
	case *annotations.HttpRule_Patch:
		return http.MethodPatch
	case *annotations.HttpRule_Custom:
		return pattern.Custom.Kind
	default:
		return http.MethodPost
	}
}

func BuildHttpPath(httpRule *annotations.HttpRule) string {
	switch pattern := httpRule.Pattern.(type) {
	case *annotations.HttpRule_Get:
		return pattern.Get
	case *annotations.HttpRule_Put:
		return pattern.Put
	case *annotations.HttpRule_Post:
		return pattern.Post
	case *annotations.HttpRule_Delete:
		return pattern.Delete
	case *annotations.HttpRule_Patch:
		return pattern.Patch
	case *annotations.HttpRule_Custom:
		return pattern.Custom.Kind
	default:
		return ""
	}
}

func BuildPathVars(path string) (res map[string]*string) {
	if strings.HasSuffix(path, "/") {
		ErrorOutput("HttpPath %s should not end with \"/\"")
	}
	pattern := regexp.MustCompile(`(?i){([a-z.0-9_\s]*)=?([^{}]*)}`)
	matches := pattern.FindAllStringSubmatch(path, -1)
	res = make(map[string]*string, len(matches))
	for _, m := range matches {
		name := strings.TrimSpace(m[1])
		if len(name) > 1 && len(m[2]) > 0 {
			res[name] = &m[2]
		} else {
			res[name] = nil
		}
	}

	return
}

type Parameter struct {
	Annotation string
	Type       string
	Name       string
	Required   bool
	HasBody    bool
}

func BuildRequestBody(message *protogen.Message, httpRule *annotations.HttpRule) Parameter {
	param := Parameter{
		Annotation: "@RequestBody",
		Type:       message.GoIdent.GoName,
		Name:       lcfirst(message.GoIdent.GoName),
		HasBody:    false,
	}
	if httpRule.Body == "*" {
		param.HasBody = true
	} else if httpRule.Body != "" {
		field, ok := LookupField(message, httpRule.Body)
		if !ok {
			ErrorOutput(fmt.Sprintf("field `%s` not found in message `%s`", httpRule.Body, message.GoIdent.GoName))
		}
		if field.Desc.Kind() != protoreflect.MessageKind {
			ErrorOutput(fmt.Sprintf("field `%s` of `%s` must be message type ", httpRule.Body, message.GoIdent.GoName))
		}

		param.Type = NewJavaKind(field).JavaString()
		param.Name = httpRule.Body
		param.HasBody = true
	}

	return param
}

func LookupField(message *protogen.Message, name string) (*protogen.Field, bool) {
	for _, field := range message.Fields {
		if field.Desc.TextName() == name {
			return field, true
		}
	}
	return nil, false
}

func BuildResponseBody(message *protogen.Message, httpRule *annotations.HttpRule) (param Parameter) {
	param = Parameter{Type: message.GoIdent.GoName, Name: lcfirst(message.GoIdent.GoName)}
	if httpRule.ResponseBody != "" && httpRule.ResponseBody != "*" {
		for _, field := range message.Fields {
			if field.Desc.TextName() == httpRule.Body {
				return Parameter{Type: field.GoName, Name: httpRule.Body}
			}
		}
		ErrorOutput(fmt.Sprintf("field %s not found in message %s", httpRule.ResponseBody, message.GoIdent.GoName))
	}
	return
}

func BuildQueryParameters(message *protogen.Message, httpRule *annotations.HttpRule, pathVars map[string]*string) []Parameter {
	if httpRule.Body == "*" {
		return []Parameter{}
	}

	lookup := func(target string) bool {
		for name, _ := range pathVars {
			if name == target {
				return true
			}
		}
		return false
	}

	var params []Parameter

	for _, field := range message.Fields {
		if lookup(field.Desc.TextName()) || field.Desc.TextName() == httpRule.Body {
			continue
		}
		descriptor := NewFieldDescriptor(field)
		annotation := fmt.Sprintf("@RequestParam(name = \"%s\")", field.Desc.TextName())
		if descriptor.IsRequired {
			annotation = fmt.Sprintf("@RequestParam(name = \"%s\", required = true)", field.Desc.TextName())
		}
		parameter := Parameter{
			Type:       descriptor.FieldType,
			Name:       descriptor.FieldName,
			Annotation: annotation,
		}
		params = append(params, parameter)
	}

	return params
}

func BuildPathParams(pathVars map[string]*string, message *protogen.Message) []Parameter {
	params := make([]Parameter, 0)
	for name, _ := range pathVars {
		var param *Parameter
		for _, f := range message.Fields {
			if string(f.Desc.Name()) == name {
				kind := NewJavaKind(f)
				param = &Parameter{Name: name, Type: kind.JavaString(), Annotation: "@PathVariable"}
				params = append(params, *param)
			}
		}
		if param == nil {
			ErrorOutput(fmt.Sprintf("%s: path param `%s` not found in `%s`", message.Desc.ParentFile().Path(), name, message.GoIdent.GoName))
		}
	}
	return params
}

func lcfirst(val string) string {
	return strings.ToLower(val[:1]) + val[1:]
}

func ucfirst(val string) string {
	return strings.ToUpper(val[:1]) + val[1:]
}

func RemoveDuplicates(slice []string) []string {
	uniqueMap := make(map[string]struct{})
	var uniqueSlice []string

	for _, item := range slice {
		if _, exists := uniqueMap[item]; !exists {
			uniqueMap[item] = struct{}{}
			uniqueSlice = append(uniqueSlice, item)
		}
	}

	return uniqueSlice
}

func TrimComments(comments string) string {
	return strings.TrimSpace(strings.Trim(comments, "/*"))
}
