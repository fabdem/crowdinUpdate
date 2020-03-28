package main

//	F.Demurger 2020-03
//
//	Update a Crowdin source file. All existing approval for the updated translations are removed.
//
//	crowdinupdate [options] <access token> <project number> <crowdin file path/name> <local file path/name>
//	or
//	crowdinupdate [options] -c <json config file> <key> <local file path/name>
//
//		Option -v version
//		Option -p <proxy url> when proxy needed.
//		Option -t <timeout in second>. Defines a timeout for each communication (r/w) with the server.
//						This doesn't provide an overall timeout!
//		Option -n no spinning thingy while we wait for the file to update (for unattended usage).
//
//		When Option -c <json config file> is used, project#, key and Crowdin path are provided in the json file
//								and depending on a key (e.g. a Perforce path).
//
//    Default timeout set in lib: 40s.
//    Returns 1 if an error occurs
//
//		config.json
//
//			"data": [
//			{
//				"key": "//project2/project2_english.txt",
//     		"projectId": 5,
//    		"authToken": "555555555555555",
//    		"destination": "/folder blah/subfolder/afile_english.vdf",
//				"extension": ".vdf"
//    		},
//		    {
//		     	"key": "//Another_project/another_file.txt",
//					....
//			}
//			]
//
//			"key" 				Needs to be unique. E.g. a perforce path and filename
//			"projectId"		Crowdin project Id
//			"authToken"		Secret authorization token to access the project
//			"destination"	Destination in Crowdin project where the file needs to go.
//			"extension"		Extension expected by Crowdin depending on the file type (e.g. vdf). The source file will be renamed with this extension unless that one is empty.
//
//
//	cross compilation Win AMD64 on linux:  env GOOS=windows GOARCH=amd64 go build crowdinupdate.go

import (
	"flag"
	"fmt"
	"io"
	"github.com/fabdem/go-crowdinv2"
	"crowdinUpdate/config"
	//"go-crowdinv2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var idx int = 0
var finishChan chan struct{}

// Spinning wheel
func animation(c *crowdin.Crowdin) {
	sequence := [...]string{"|", "/", "-", "\\"}

	for {
		select {
		default:
			str := fmt.Sprintf("%s", sequence[idx])
			fmt.Printf("%s%s", str, strings.Repeat("\b", len(str)))
			// fmt.Printf("%s %d", sequence[idx], c.GetProjectId2())
			idx = (idx + 1) % len(sequence)
			amt := time.Duration(100)
			time.Sleep(time.Millisecond * amt)

		case <-finishChan:
			return
		}
	}
}

func main() {

	var versionFlg bool
	var proxy string
	var timeoutsec int
	var nospinFlg bool
	var uRL string
	var debug string
	var conf string
	var updateMode string

	const usageVersion = "Display Version"
	const usageProxy = "Use a proxy - followed with url"
	const usageTimeout = "Set the build timeout in seconds (default 50s)."
	const usageNospin = "No spinning |"
	const usageUrl = "Specify the API URL"
	const usageDebug = "Store Debug info in a file followed with path and filename"
	const usageConf = "Config in json file"
	const usageUpdate = "Define the type of update. Has to be either:\n   - clear_translations_and_approvals\n   - keep_translations\n   - keep_translations_and_approvals\n"

	checkFlags := flag.NewFlagSet("check", flag.ExitOnError)

	checkFlags.BoolVar(&versionFlg, "version", false, usageVersion)
	checkFlags.BoolVar(&versionFlg, "v", false, usageVersion+" (shorthand)")
	checkFlags.IntVar(&timeoutsec, "timeout", 50, usageTimeout)
	checkFlags.IntVar(&timeoutsec, "t", 50, usageTimeout+" (shorthand)")
	checkFlags.StringVar(&proxy, "proxy", "", usageProxy)
	checkFlags.StringVar(&proxy, "p", "", usageProxy+" (shorthand)")
	checkFlags.BoolVar(&nospinFlg, "nospin", false, usageNospin)
	checkFlags.BoolVar(&nospinFlg, "n", false, usageNospin+" (shorthand)")
	checkFlags.StringVar(&uRL, "url", "", usageNospin)
	checkFlags.StringVar(&uRL, "u", "", usageUrl+" (shorthand)")
	checkFlags.StringVar(&debug, "debug", "", usageDebug)
	checkFlags.StringVar(&debug, "d", "", usageDebug+" (shorthand)")
	checkFlags.StringVar(&conf, "conf", "", usageConf)
	checkFlags.StringVar(&conf, "c", "", usageConf+" (shorthand)")
	checkFlags.StringVar(&updateMode, "update", "", usageUpdate)
	checkFlags.StringVar(&updateMode, "a", "", usageUpdate+" (shorthand)")
	checkFlags.Usage = func() {
		fmt.Print("Usage:")
		fmt.Printf(" %s [options] <access token> <project number> <crowdin file path/name> <local file path/name>\n", os.Args[0])
		fmt.Print("  or")
		fmt.Printf(" %s [options]  -c <json config file> <P4 file path/name> <local file path/name>\n", os.Args[0])
		checkFlags.PrintDefaults()
	}

	// Check parameters
	checkFlags.Parse(os.Args[1:])

	if versionFlg {
		fmt.Printf("Version %s\n", "2020-03  v1.2.4")
		os.Exit(0)
	}

	var token, crowdinFile string
	var projectId int
	var ext string
	var err error

	index := len(os.Args)

	// Path and name of local file name to send to Crowdin
	localFile := os.Args[index-1]
	// fmt.Printf("\ndebug  localFile: %s\n", localFile)

	f, err := os.Open(localFile) // Check if source fileExists
	if err != nil {
		fmt.Printf("\ncrowdinupdate() - can't find source file: %s %v\n", localFile, err)
		os.Exit(1)
	}
	f.Close()

	if conf != "" { // A json file is provided to define the Crowdin projects info
		p4File := os.Args[index-2]
		json, err := config.New(conf)
		if err != nil {
			fmt.Printf("\ncrowdinupdate() - can't find json file: %s %v\n", conf, err)
			os.Exit(1)
		}
		res, err := json.GetValue(p4File)
		if err != nil {
			fmt.Printf("\ncrowdinupdate() - json formatting issue: %v\n", err)
			os.Exit(1)
		}
		// fmt.Printf("\ndebug  conf file read: %s\n", conf)

		uRL = res.Apiurl
		projectId = res.ProjectId
		token = res.AuthToken
		crowdinFile = res.Destination
		ext = res.Extension

		newName := changeNameExt(localFile, ext) // Change the filename extension if needed

		if newName != localFile { // If file names differ then create a copy with newname
			// fmt.Printf("Copying %s to %s", localFile, newName)
			if copyFile(localFile, newName) != nil {
				fmt.Printf("\ncrowdinupdate() - file copy failed: %v\n", err)
				os.Exit(1)
			}
		}

		localFile = newName

	} else { // No json file, access to Crowdin projects is defined on the cmd line
		// Parse the command parameters
		token = os.Args[index-4]
		projectId, err = strconv.Atoi(os.Args[index-3])
		if err != nil {
			fmt.Printf("\ncrowdinupdate() - ProjectId needs to be a number %s", err)
			os.Exit(1)
		}

		// Path and name of file to update in Crowdin. Stored in a slice.
		crowdinFile = os.Args[index-2]
	}

	// Create a connection to Crowdin
	crowdin.SetTimeouts(5, timeoutsec) // Not ideal: forced to use  the r/w timeout to enforce the application timeout :(
	api, err := crowdin.New(token, projectId, uRL, proxy)
	if err != nil {
		fmt.Printf("\ncrowdinupdate() - connection problem %s\n", err)
		os.Exit(1)
	}

	if len(debug) > 0 {
		logFile, err := os.Create(debug)
		if err != nil {
			fmt.Printf("\ncrowdinupdate() - Can't create debug file %s %v", debug, err)
			os.Exit(1)
		}
		api.SetDebug(true, logFile)
	}

	if !nospinFlg { // Check if we need to spin the '|'
		finishChan = make(chan struct{})
		go animation(api)
	}

	// Update file in Crowdin project
	fileId, err := api.Update(crowdinFile, localFile, updateMode)
	if err != nil {
		fmt.Printf("\ncrowdinupdate() - update error %s\n\n", err)
		os.Exit(1)
	}

	// Get revision details
	revisions, err := api.ListFileRevisions(&crowdin.ListFileRevisionsOptions{Limit: 500}, fileId)
	if err != nil {
		fmt.Printf("\ncrowdinupdate() - Read revision details error %s\n\n", err)
		os.Exit(1)
	}

	r := revisions.Data[len(revisions.Data)-1]

	fmt.Printf("\nOperation successful - Revision#: %v", r.Data.Id)
	fmt.Printf("\n  Added   Lines	: %d  (%d words)", r.Data.Info.Added.Strings, r.Data.Info.Added.Words)
	fmt.Printf("\n  Deleted Lines	: %d  (%d words)", r.Data.Info.Deleted.Strings, r.Data.Info.Deleted.Words)
	fmt.Printf("\n  Updated Line	: %d  (%d words)", r.Data.Info.Updated.Strings, r.Data.Info.Updated.Words)
	fmt.Print("\n")

	if !nospinFlg {
		close(finishChan) // Stop animation
	}
}

// Change the extension of a file name - doesn't do the actual file renaming part
//  name: path and file name
//  ext: extension. If empty the fonction returns the file name unchanged.
func changeNameExt(name string, ext string) string {
	if ext == "" {
		return name
	}

	if nameext := filepath.Ext(name); nameext == "" { // File name with no ext
		return name + ext
	} else { // Filename with an extension
		return name[0:strings.LastIndex(name, nameext)] + ext
	}

	return "" // Default empty string
}

// Copy a 'source' file to 'destination'
func copyFile(source, destination string) error {
	from, err := os.Open(source)
	if err != nil {
		return err
	}
	defer from.Close()

	to, err := os.OpenFile(destination, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		return err
	}

	return nil
}
