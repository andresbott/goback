package config

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Unmarshal will recursively navigate the struct using reflection and populate the fields based on the
// flattened config representation
func (c *Config) Unmarshal(payload any) error {
	c.info("unmarshalling config into struct")
	_, err := c.unmarshal(reflect.ValueOf(payload), "")
	if err != nil {
		return err
	}
	return nil
}

const sep = "."

// internal recursive unmarshal function, it returns true if any change was made to the passed pointer
//
//nolint:gocognit
func (c *Config) unmarshal(item reflect.Value, prefix string) (bool, error) {
	if len(prefix) > 0 {
		prefix += sep
	}

	// make sure we always pass in a pointer to a struct
	if item.Kind() != reflect.Ptr {
		return false, fmt.Errorf("passed argument is not a pointer")
	}
	item = item.Elem()
	if item.Kind() != reflect.Struct {
		return false, fmt.Errorf("passed argument is not a pointer to a struct")
	}

	changed := false
	for i := 0; i < item.NumField(); i++ {
		valueField := item.Field(i)
		typeField := item.Type().Field(i)

		fieldName := prefix + typeField.Name

		tag := sanitizeTag(typeField.Tag.Get("config"))
		if tag != "" {
			fieldName = prefix + tag
		}

		switch valueField.Kind() {
		case reflect.Bool,
			reflect.String,
			reflect.Float64,
			reflect.Float32,
			reflect.Int:

			ch, err := c.setValue(valueField, fieldName)
			if err != nil {
				return changed, err
			}
			if ch {
				changed = true
			}

		case reflect.Slice:
			ch, err := c.setSlice(valueField, fieldName)
			if err != nil {
				return changed, err
			}
			if ch {
				changed = true
			}

		case reflect.Struct:
			ch, err := c.unmarshal(valueField.Addr(), fieldName)
			if err != nil {
				return changed, err
			}
			if ch {
				changed = true
			}

		default:
			return changed, fmt.Errorf("unhandled type: \"%s\" in struct", valueField.Kind())
		}
	}

	return changed, nil
}

func sanitizeTag(in string) string {
	// todo: maybe if we use more struct tags we need to split and so on
	return strings.TrimSpace(in)
}

var boolValues = []string{
	"1",
	"t",
	"T",
	"TRUE",
	"true",
	"True",
	"0",
	"f",
	"F",
	"FALSE",
	"false",
	"False",
}

// setValue takes a single reflect.value field from a struct to be unmarshalled and sets the value
// in order of precedence it checks first if the field name is present in the flattened map
// and then overrides with ENVs if any is found for the same key
//
//nolint:gocognit,nestif
func (c *Config) setValue(valueField reflect.Value, fieldName string) (bool, error) {

	var val reflect.Value
	changed := false
	if c.flatData[fieldName] != nil {
		val = reflect.ValueOf(c.flatData[fieldName])
		c.Debug(fmt.Sprintf("setting value of field \"%s\" from config", fieldName))
		changed = true
	}
	// check ENV
	envName := fieldName
	if c.envPrefix != "" {
		envName = c.envPrefix + "_" + fieldName
	}
	envName = strings.ReplaceAll(envName, sep, envSep)
	envName = strings.ToUpper(envName)
	envVal := os.Getenv(envName)

	if c.loadEnvs && envVal != "" {
		switch valueField.Kind() {
		case reflect.Int:
			data, err := strconv.Atoi(envVal)
			if err != nil {
				return changed, fmt.Errorf("unable to convert env to int %s", err)
			}
			val = reflect.ValueOf(data)
			changed = true
			c.Debug(fmt.Sprintf("setting value of field \"%s\" from ENV", fieldName))
		case reflect.Bool:
			if slices.Contains(boolValues, envVal) {
				val = reflect.ValueOf(true)
				changed = true
				c.Debug(fmt.Sprintf("setting value of field \"%s\" from ENV", fieldName))
			}
		case
			reflect.String:
			val = reflect.ValueOf(envVal)
			changed = true
			c.Debug(fmt.Sprintf("setting value of field \"%s\" from ENV", fieldName))
		case
			reflect.Float64:
			data, err := strconv.ParseFloat(envVal, 64)
			if err != nil {
				return changed, fmt.Errorf("unable to convert env to float64 %s", err)
			}
			val = reflect.ValueOf(data)
			changed = true
			c.Debug(fmt.Sprintf("setting value of field \"%s\" from ENV", fieldName))
		case
			reflect.Float32:
			data, err := strconv.ParseFloat(envVal, 32)
			if err != nil {
				return changed, fmt.Errorf("unable to convert env to float32 %s", err)
			}
			val = reflect.ValueOf(data)
			changed = true
			c.Debug(fmt.Sprintf("setting value of field \"%s\" from ENV", fieldName))
		default:
			c.Debug(fmt.Sprintf("NOT setting the value of \"%s\" becasue it is not a supported type", fieldName))
		}
	}

	if changed {
		// load file if string starts with @
		if valueField.Kind() == reflect.String {
			strVal, err := c.fileOrString(val.String(), fieldName)
			if err != nil {
				return changed, err
			}
			val = reflect.ValueOf(strVal)
		}

		fn := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("reflect panniked while setting value to field: %s (probably unexported)", fieldName)
				}
			}()
			valueField.Set(val)
			return
		}
		err := fn()
		if err != nil {
			return changed, err
		}

	}
	return changed, nil
}

func loadFileContent(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// setSlice takes a single reflect.value of kind slice from a struct to be unmarshalled and sets the value of the slice
// in order of precedence it checks first if the field name is present in the flattened map
// and then overrides with ENVs if any is found for the same key
func (c *Config) setSlice(valueField reflect.Value, fieldName string) (bool, error) {

	changed := false

	switch valueField.Type().Elem().Kind() {
	case reflect.Bool,
		reflect.String,
		reflect.Float64,
		reflect.Float32,
		reflect.Int:

		i := 0
		for {
			localName := fieldName + "." + strconv.Itoa(i)

			// Create a new instance of the struct
			newInstance := reflect.New(valueField.Type().Elem()).Elem()

			ch, err := c.setValue(newInstance, localName)
			if err != nil {
				return changed, err
			}
			if ch {
				changed = true
			} else {
				// if no change was made to the newInstance struct then we can exit the loop
				break
			}

			newSlice := reflect.Append(valueField, reflect.ValueOf(newInstance.Interface()))
			valueField.Set(newSlice)
			i++
		}

	case reflect.Struct:
		i := 0
		for {
			localName := fieldName + "." + strconv.Itoa(i)

			// Create a new instance of the struct
			newInstance := reflect.New(valueField.Type().Elem()).Elem()

			// use the unmarshal function to populate the created struct
			ch, err := c.unmarshal(newInstance.Addr(), localName)
			if err != nil {
				return changed, err
			}
			if ch {
				changed = true
			} else {
				// if no change was made to the newInstance struct then we can exit the loop
				break
			}

			newSlice := reflect.Append(valueField, reflect.ValueOf(newInstance.Interface()))
			valueField.Set(newSlice)
			i++
		}

	}
	return changed, nil
}
