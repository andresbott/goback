package config

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

type Config struct {
	loadEnvs  bool
	envPrefix string
	flatData  map[string]any
	subset    string
	writter   func(level, msg string)
}

func Load(opts ...any) (*Config, error) {

	c := Config{
		//data:     map[string]interface{}{},
		flatData: map[string]any{},
	}
	cl, err := newCfgLoader(opts)
	if err != nil {
		return nil, err
	}
	// set writer
	if cl.writer != nil {
		c.writter = cl.writer.Fn
	}

	// load defaults
	if cl.def != nil {
		c.info("loading default values")
		err := flattenStruct(cl.def.Item, c.flatData)
		if err != nil {
			return nil, err
		}
	}

	// enable envs
	if cl.env != nil {
		c.info(fmt.Sprintf("using ENVS with prefix \"%s\"", cl.env.Prefix))
		c.envPrefix = cl.env.Prefix
		c.loadEnvs = true
	}

	// load from file
	if cl.file != nil {
		extType := fileType(cl.file.Path)
		if extType == ExtUnsupported {
			return nil, fmt.Errorf("file %s is of unsuporeted type", cl.file.Path)
		}
		c.info(fmt.Sprintf("loading config from FILE: \"%s\"", cl.file.Path))
		byt, err := os.ReadFile(cl.file.Path)
		if err != nil {
			return nil, err
		}

		data, err := readCfgBytes(byt, extType)
		if err != nil {
			return nil, err
		}
		flatten("", data, c.flatData)
	}

	// implicit unmarshal call
	if cl.unmar != nil {
		err := c.Unmarshal(cl.unmar.Item)
		if err != nil {
			return nil, err
		}
	}

	return &c, nil
}

// Defaults enables to provide a set of default values to the configuration
type Defaults struct {
	Item any
}

// CfgFile enables config to be loaded from a single file
type CfgFile struct {
	Path string
}

// CfgDir enables config to be loaded from a conf.d directory,
// note that directory values will take precedence over single file
type CfgDir struct {
	Path string
}

// EnvVar enables to load config using an env vars
// note that Envs will take precedence over file persisted values
type EnvVar struct {
	Prefix string
}

// Unmarshal is an implicit call to unmarshal when calling the Load function
// if you intend to simply load + unmarshal, with this item you can simplify
// one function call
type Unmarshal struct {
	Item any
}

const (
	InfoLevel  = "info"
	DebugLevel = "debug"
	WarnLevel  = "warn"
)

// Writer allows to print information about what the config is doing
// it will write two levels info and debug
type Writer struct {
	Fn func(level, msg string)
}

// cfgLoader holds references to options to control the order of precedence
type cfgLoader struct {
	def  *Defaults
	file *CfgFile
	//dir    *CfgDir
	env    *EnvVar
	writer *Writer
	unmar  *Unmarshal
}

func newCfgLoader(opts []any) (cfgLoader, error) {

	cl := cfgLoader{}

	for _, opt := range opts {
		switch item := opt.(type) {
		case Defaults:
			cl.def = &item
		case CfgFile:
			cl.file = &item
		//case CfgDir:
		//	// TODO implemnt
		//	spew.Dump("CfgDir: TODO implement")
		case EnvVar:
			cl.env = &item
		case Writer:
			cl.writer = &item
		case Unmarshal:
			cl.unmar = &item
		case []any:
			return cl, fmt.Errorf("wrong options payload: [][]any, only pass an array of options")
		}
	}
	return cl, nil

}

const envSep = "_"

func (c *Config) GetString(fieldName string) (string, error) {
	// check ENV firs
	envName := fieldName
	if c.envPrefix != "" {
		envName = c.envPrefix + "_" + fieldName
	}
	envName = strings.ReplaceAll(envName, sep, envSep)
	envName = strings.ToUpper(envName)
	envVal := os.Getenv(envName)

	if c.loadEnvs && envVal != "" {
		return c.fileOrString(envVal, fieldName)
	}

	val, ok := c.flatData[fieldName]
	if ok {
		switch item := val.(type) {
		case map[string]interface{}:
			return "", nil
		case string:
			return c.fileOrString(item, fieldName)
		default:
			return fmt.Sprintf("%v", val), nil
		}
	}
	return "", fmt.Errorf("config key not found")
}

// fileOrString checks if of a string is supposed to load a file if it starts with @
// if so, it loads the file and returns the content otherwise it returns the original string
func (c *Config) fileOrString(in, fieldName string) (string, error) {
	if strings.HasPrefix(in, "@") {
		p := strings.TrimPrefix(in, "@")
		strVal, err := loadFileContent(p)
		if err != nil {
			return "", err
		}
		c.Debug(fmt.Sprintf("setting value of field \"%s\" from file content of: %s ", fieldName, p))
		return strVal, nil
	}
	return in, nil
}

// Subset returns a config that only handles a subset of the overall config
func (c *Config) Subset(key string) *Config {
	newC := Config{
		loadEnvs:  c.loadEnvs,
		envPrefix: c.envPrefix,
		subset:    c.subset + sep + key,
		//data:      c.data,
		flatData: c.flatData,
	}

	return &newC
}

func (c *Config) info(msg string) {
	if c.writter != nil {
		c.writter(InfoLevel, msg)
	}
}

func (c *Config) Debug(msg string) {
	if c.writter != nil {
		c.writter(DebugLevel, msg)
	}
}

// flatten takes a nested map[string]any and transforms it into a flat map[string]any, where the keys of the nested
// items are concatenated by sep. If the item is already present in the destination it will be overwritten
func flatten(prefix string, src map[string]any, dest map[string]any) {
	// got from: https://stackoverflow.com/questions/64419565/how-to-efficiently-flatten-a-map
	if len(prefix) > 0 {
		prefix += sep
	}
	for k, v := range src {
		switch child := v.(type) {
		case map[string]interface{}:
			flatten(prefix+k, child, dest)
		case []interface{}:
			for i := 0; i < len(child); i++ {
				switch child[i].(type) {
				case map[string]interface{}:
					flatten(prefix+k+sep+strconv.Itoa(i), child[i].(map[string]interface{}), dest)
				default:
					dest[prefix+k+sep+strconv.Itoa(i)] = child[i]
				}
			}
		default:
			dest[prefix+k] = v
		}
	}
}

// flattenStruct takes a struct or struct pointer and maps it into a flat map[string]any
func flattenStruct(src any, dest map[string]any) error {
	// make sure we always pass in a pointer to a struct
	item := reflect.ValueOf(src)
	if item.Kind() != reflect.Ptr && item.Kind() != reflect.Struct {
		return fmt.Errorf("passed src is not a pointer or struct")
	}

	if item.Kind() == reflect.Ptr {
		item = item.Elem()
		if item.Kind() != reflect.Struct {
			return fmt.Errorf("passed argument is not a pointer to a struct")
		}
	}
	flattenStructRec("", item, dest)
	return nil
}

// flattenStructRec is the inner recursive step for flattenStruct
func flattenStructRec(prefix string, item reflect.Value, dest map[string]any) {
	if len(prefix) > 0 {
		prefix += sep
	}

	for i := 0; i < item.NumField(); i++ {
		valueField := item.Field(i)
		typeField := item.Type().Field(i)

		fieldName := prefix + typeField.Name

		tag := sanitizeTag(typeField.Tag.Get("config"))
		if tag != "" {
			fieldName = prefix + tag
		}

		// don't put zero values into the destination map
		if valueField.IsZero() {
			continue
		}
		switch valueField.Kind() {
		case reflect.Bool:
			dest[fieldName] = valueField.Bool()
		case reflect.String:
			dest[fieldName] = valueField.String()
		case reflect.Float64:
			dest[fieldName] = valueField.Float()
		case
			reflect.Float32:
			dest[fieldName] = int32(valueField.Float())
		case
			reflect.Int64:
			dest[fieldName] = int64(valueField.Int())
		case
			reflect.Int:
			dest[fieldName] = int(valueField.Int())
		case reflect.Slice:

			for j := 0; j < valueField.Len(); j++ {
				childValue := valueField.Index(j)
				switch childValue.Kind() {
				case reflect.Struct:
					flattenStructRec(fieldName+sep+strconv.Itoa(j), childValue, dest)
				default:
					switch childValue.Kind() {

					case reflect.Bool:
						dest[fieldName+sep+strconv.Itoa(j)] = childValue.Bool()
					case reflect.String:
						dest[fieldName+sep+strconv.Itoa(j)] = childValue.String()
					case reflect.Float64:
						dest[fieldName+sep+strconv.Itoa(j)] = childValue.Float()
					case
						reflect.Float32:
						dest[fieldName+sep+strconv.Itoa(j)] = int32(childValue.Float())
					case
						reflect.Int64:
						dest[fieldName+sep+strconv.Itoa(j)] = int64(childValue.Int())
					case
						reflect.Int:
						dest[fieldName+sep+strconv.Itoa(j)] = int(childValue.Int())
					}
				}
			}

		case reflect.Struct:
			flattenStructRec(fieldName, valueField, dest)
		}

	}
}

// readCfgBytes takes a []byte, normally from reading a file, and will parse it's content depending
// on the extension passed in ext
// it returns a map[string]any
func readCfgBytes(bytes []byte, ext string) (map[string]any, error) {
	var data map[string]any

	if ext == ExtYaml {
		err := yaml.Unmarshal(bytes, &data)
		if err != nil {
			return nil, err
		}
		return data, nil
	}

	if ext == ExtJson {
		err := json.Unmarshal(bytes, &data)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	return nil, fmt.Errorf("unsuported file type")

}

const (
	ExtYaml        = "YAML"
	ExtJson        = "JSON"
	ExtUnsupported = "unsupported"
)

func fileType(fpath string) string {
	filename := filepath.Base(fpath)
	extension := strings.TrimPrefix(filepath.Ext(filename), ".")
	extension = strings.ToUpper(extension)
	switch extension {
	case ExtYaml:
		return ExtYaml
	case ExtJson:
		return ExtJson
	default:
		return ExtUnsupported
	}
}
