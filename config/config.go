package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

type Config struct {
	filename string
	conf     configFile
}

type FileAccess struct {
	Apiurl      string
	ProjectId   int
	AuthToken   string
	Destination string
	Extension   string
}

type ProjectDef struct {
	Name		string 	`json:"name"`
	Apiurl      string 	`json:"apiurl"`
	ProjectId   int    	`json:"projectId"`
	AuthToken   string 	`json:"authToken"`		
}

type FileDef struct {
	Key         string 	`json:"key"`	// e.g. a path + filename
	ProjectName string 	`json:"project_name"`
	Destination string 	`json:"destination"`
	Extension   string 	`json:"extension"`
}

type configFile struct {
	Projects	[]ProjectDef	`json:"projects"`
	Files	 	[]FileDef		`json:"files"`
}


var c 	Config					// config stored in memeory
var prj map[string]ProjectDef	// map to simplify access to project details



// New()
// Create a new instance.
// Open the json file and load its content in memory.
// 	Parameter:
//		- path and name of the file
//	Returns:
//		- err != null in case of error
//		- pointer to instance
func New(jsonfilename string) (*Config, error) {
	// var err error

	if !fileExists(jsonfilename) {
		return nil, errors.New(fmt.Sprintf("package config - Can't find file %s", jsonfilename))
	}

	c := &Config{}
	c.filename = jsonfilename

	// Try to load a json file
	jsonFile, err := os.Open(jsonfilename)
	if err != nil {
		fmt.Printf("Error - problem opening %s\n%s\n", jsonfilename, err)
		return nil, errors.New(fmt.Sprintf("package config - Can't open file %s", jsonfilename))
	}
	// defer the closing
	defer jsonFile.Close()

	// read the file in a byte slice.
	buffer, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("package config - Issue reading %s %v", jsonfilename, err))
	}

	// fmt.Printf("file read: %s", buffer)
	// fmt.Printf("struct: %v", c.conf)

	// Unmarshal buffer which contains our
	// jsonFile's content into the structure
	err = json.Unmarshal(buffer, &c.conf)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("package config - Issue unmarshalling json %v", err))
	}
	
	// Move project details into a map to simplify access to data
	prj = make(map[string]ProjectDef)
	for _, v := range c.conf.Projects {
		prj[v.Name] = v
	}
	if len(prj) <= 0 {
		return nil, errors.New(fmt.Sprintf("package config - at least one project needs to be defined"))
	}
	
	return c, nil
}

// GetDetails()
//	Get the project details corresponding to a key (e.g. filename)
//  
//
// 	Parameter:
//		- Unique key
//	Returns:
//		- err != null if fails to find a corresponding value
//   	- Array of FileAccess. Multiple lines if there are multiple destinations.
//
func (c *Config) GetValue(key string) ([]FileAccess list, error err) {

	for _, f := range c.conf.Files {
		if f.Key == key { // found it
			var r FileAccess
			r.Apiurl		= prj[v.Name].Apiurl
			r.ProjectId 	= prj[v.Name].ProjectId
			r.AuthToken 	= prj[v.Name].AuthToken
			r.Destination	= v.Destination
			r.Extension		= v.Extension
			if len(r.Extension) > 0 {
				if r.Extension == "." { // Equivalent to no extension
					r.Extension = ""
				} else {
					if strings.Index(r.Extension, ".") < 0 { // add a '.' before ext if there's none
						r.Extension = "." + r.Extension
					}
				}
			}
			list = append(list,r)
		}
	}
	if len(list) > 0 {
		return list, nil
	}
	return list, errors.New(fmt.Sprintf("package config - Can't find a project for %s", key))
}

// fileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
