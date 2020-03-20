package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/ldap.v3"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"gopkg.in/yaml.v2"
)

// Note: struct fields must be public in order for unmarshal to
// correctly populate the data.
type OpaConfig struct {
	Services []struct {
		Name string `yaml:"name"`
		URL  string `yaml:"url"`
	} `yaml:"services"`
	Labels struct {
		App         string `yaml:"app"`
		Region      string `yaml:"region"`
		Environment string `yaml:"environment"`
	} `yaml:"labels"`
	Bundles []struct {
		Name     string `yaml:"name"`
		Service  string `yaml:"service"`
		Resource string `yaml:"resource"`
		Pooling  struct {
			MinDelaySeconds int `yaml:"min_delay_seconds"`
			MaxDelaySeconds int `yaml:"max_delay_seconds"`
		} `yaml:"polling"`
	} `yaml:"bundles"`
	DecisionLogs struct {
		Console bool `yaml:"console"`
	} `yaml:"decision_logs"`
}

type CloudBuild struct {
	Steps []struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Args       []string `json:"args,omitempty"`
		Env        []string `json:"env,omitempty"`
		Entrypoint string   `json:"entrypoint,omitempty"`
	} `json:"steps"`
	Timeout       string `json:"timeout"`
	Substitutions struct {
		OBDOCKER   string `json:"_OB_DOCKER"`
		OBBINARIES string `json:"_OB_BINARIES"`
	} `json:"substitutions"`
	Options struct {
		WorkerPool string `json:"workerPool"`
	} `json:"options"`
}

type LdapGroupEntitlements struct {
	Version    string `yaml:"version"`
	LdapGroups []struct {
		Name  string `yaml:"name"`
		Roles []struct {
			Name              string `yaml:"name"`
			EntitlementGroups []struct {
				Name         string   `yaml:"name"`
				Entitlements []string `yaml:"entitlements"`
			} `yaml:"entitlement_groups"`
		} `yaml:"roles"`
	} `yaml:"ldap_groups"`
}

func main() {
	// Print LDAP search results
	// SearchUsers()
	//SearchGroups()

	//FetchGitFile()
	UpdateGitFile()
	//makeTempRepo()

	// ParseYMLFile()

	//filePaths := []string{".manifest", "uam2/entitlements/opa-policy.rego"}
	//fmt.Printf("Paths 2 %v\n", filePaths)
	//
	//home, _ := os.Getwd() //os.UserHomeDir()
	//fmt.Printf("home1: %v\n", home)
	//err := os.Chdir(filepath.Join(home, "tempOpa/policy"))
	//if err != nil {
	//	panic(err)
	//}
	//mydir, _ := os.Getwd()
	//fmt.Printf("home2: %v\n", mydir)
	//
	//// TAR the file
	//err = tartarWalk("bundle-policy.tar.gz", filePaths)
	//fmt.Printf("err: %v\n", err)
	//
	//err = os.Chdir(filepath.Join(home, "."))
	//if err != nil {
	//	panic(err)
	//}
	//mydir, _ = os.Getwd()
	//fmt.Printf("home3: %v\n", mydir)
}

// tartarWalk walks paths to create tar file tarName
func tartarWalk(tarName string, paths []string) (err error) {
	tarFile, err := os.Create(tarName)
	if err != nil {
		return err
	}
	defer func() {
		err = tarFile.Close()
	}()

	absTar, err := filepath.Abs(tarName)
	if err != nil {
		return err
	}

	// enable compression if file ends in .gz
	tw := tar.NewWriter(tarFile)
	if strings.HasSuffix(tarName, ".gz") || strings.HasSuffix(tarName, ".gzip") {
		gz := gzip.NewWriter(tarFile)
		defer gz.Close()
		tw = tar.NewWriter(gz)
	}
	defer tw.Close()

	// walk each specified path and add encountered file to tar
	for _, path := range paths {
		// validate path
		path = filepath.Clean(path)
		absPath, err := filepath.Abs(path)
		fmt.Printf("absPath: %v\n", absPath)
		if err != nil {
			fmt.Println(err)
			continue
		}
		if absPath == absTar {
			fmt.Printf("tar file %s cannot be the source\n", tarName)
			continue
		}
		if absPath == filepath.Dir(absTar) {
			fmt.Printf("tar file %s cannot be in source %s\n", tarName, absPath)
			continue
		}

		walker := func(file string, finfo os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// fill in header info using func FileInfoHeader
			hdr, err := tar.FileInfoHeader(finfo, finfo.Name())
			if err != nil {
				return err
			}

			relFilePath := file
			if filepath.IsAbs(path) {
				relFilePath, err = filepath.Rel(path, file)
				if err != nil {
					return err
				}
			}
			// ensure header has relative file path
			hdr.Name = relFilePath

			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			// if path is a dir, dont continue
			if finfo.Mode().IsDir() {
				return nil
			}

			// add file to tar
			srcFile, err := os.Open(file)
			if err != nil {
				return err
			}
			defer srcFile.Close()
			_, err = io.Copy(tw, srcFile)
			if err != nil {
				return err
			}
			return nil
		}

		// build tar
		if err := filepath.Walk(path, walker); err != nil {
			fmt.Printf("failed to add %s to tar: %s\n", path, err)
		}
	}
	return nil
}

func makeTempRepo() (*git.Repository, billy.Filesystem, error) {
	s := memory.NewStorage()
	f := memfs.New()
	r, err := git.Init(s, f)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create in-memory repo: %v", err)
	}

	readme, err := f.Create("/README.md")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create a file in repository: %v", err)
	}
	readme.Write([]byte("Hello world"))

	w, _ := r.Worktree()
	_, err = w.Add("/README.md")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to add file to repository: %v", err)
	}
	commit, err := w.Commit("Golang Test Commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Ashish Tripathi",
			Email: "john@does.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to commit file to repository: %v", err)
	}

	obj, err := r.CommitObject(commit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get commit: %v", err)
	}
	if obj == nil {
		return nil, nil, fmt.Errorf("commit object is nil")
	}

	fmt.Printf("%v\n", obj)

	// Add a new remote, with the default fetch refspec
	fmt.Println("git remote add example https://github.com/ashish246/GolangGitExample.git")
	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://ashish246:Ashish1@github@github.com/ashish246/GolangGitExample.git"},
	})
	if err != nil {
		fmt.Printf("Error Create Remote: %v\n", err)
	}
	// List remotes from a repository
	//fmt.Println("git remotes -v")

	// > git show-ref
	//fmt.Println("git show-ref")
	var referenceList []config.RefSpec
	refs, err := r.References()
	_ = refs.ForEach(func(ref *plumbing.Reference) error {
		referenceList = append(referenceList,
			config.RefSpec(ref.Name()+":"+ref.Name()))
		return fmt.Errorf("References: %v", ref.Name())
	})
	fmt.Println("References: ", referenceList)

	//fmt.Println("git push ")
	err = r.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   referenceList,
	})
	fmt.Printf("Error Push: %v\n", err)

	return r, f, nil
}

func UpdateGitFile() {
	//CheckArgs("<url>", "<directory>")
	// url := "https://github.service.anz/csp/opa-bundling-service"
	url := "https://github.com/ashish246/GolangGitExample.git"
	//directory := "."

	// fs doesn't have .git and all git stuff is in your Storer
	storage := memory.NewStorage()
	//fs contains all the files of the Git Repo
	fs := memfs.New()
	r, err := git.Clone(storage, fs, &git.CloneOptions{
		Auth: &http.BasicAuth{
			Username: "ashish246",
			Password: "Ashish1@github",
		},
		URL:           url,
		Progress:      os.Stdout,
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName("release"),
	})
	if err != nil {
		fmt.Errorf("Error: %s", err)
	}

	ref, err := r.Head()
	if err != nil {
		_ = fmt.Errorf("GIT repo HEAD not found: %v\n", err)
	}
	fmt.Println("Head ref before checkout: " + ref.Name())

	//err = r.Fetch(&git.FetchOptions{
	//	RemoteName: "origin",
	//	RefSpecs:   []config.RefSpec{"refs/heads/master:refs/heads/master", "refs/heads/release:refs/heads/release", "HEAD:refs/heads/HEAD"},
	//	Auth: &http.BasicAuth{
	//		Username: "ashish246",
	//		Password: "Ashish1@github",
	//	},
	//})
	//if err != nil {
	//	fmt.Println(err)
	//}

	// Switch the branch to the configured one
	var w *git.Worktree
	w, err = r.Worktree()
	//err = w.Checkout(&git.CheckoutOptions{
	//	Branch: plumbing.NewBranchReferenceName("release"),
	//	Create: true,
	//})
	//if err != nil {
	//	_ = fmt.Errorf("GIT Checkout command failed: %v\n", err)
	//}
	//
	//// Check the current HEAD of the cloned repo. Must be the configured branch
	//ref, err = r.Head()
	//if err != nil {
	//	_ = fmt.Errorf("GIT repo HEAD not found: %v\n", err)
	//}
	//fmt.Println("Head ref after checkout: " + ref.Name())

	// Pull the new checked out branch for latest changes
	//err = w.Pull(&git.PullOptions{RemoteName: "origin"})
	//if err != nil && err != git.NoErrAlreadyUpToDate {
	//	_ = fmt.Errorf("GIT Pull command failed: %v\n", err)
	//}

	// w, _ := r.Worktree()

	// OPEN opens the file ONLY for reading. To override file - os.OpenFile("/path/to/file", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	readme, err := fs.OpenFile("README.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Errorf("failed to open a file in repository: %v", err)
	}

	// Write mode content to the README file
	//readme.Write([]byte("\nHello world 1\n"))
	writer := bufio.NewWriter(readme)
	_, err = writer.WriteString("\n 13-----\n")
	//fmt.Printf("Wrote %d bytes\n", n4)
	writer.Flush()
	// Read the content
	readme1, err := fs.Open("README.md")
	reader := bufio.NewReader(readme1)
	_, err = ioutil.ReadAll(reader)
	// fmt.Printf("Read Text: %s\n", string(yamlFile))

	// check the status of the repo
	status, err := w.Status()
	fmt.Printf("Status: %v\n", status.IsClean())

	_, err = w.Add("README.md")
	if err != nil {
		fmt.Errorf("failed to add file to repository: %v", err)
	}
	commit, err := w.Commit("Golang Test Commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Ashish Tripathi",
			Email: "john@does.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		fmt.Errorf("failed to commit file to repository: %v", err)
	}

	obj, err := r.CommitObject(commit)
	if err != nil {
		fmt.Errorf("failed to get commit: %v", err)
	}
	if obj == nil {
		fmt.Errorf("commit object is nil")
	}

	fmt.Printf("%v\n", obj)

	//fmt.Println("git show-ref")
	//var referenceList []config.RefSpec
	//refs, err := r.References()
	//_ = refs.ForEach(func(ref *plumbing.Reference) error {
	//	referenceList = append(referenceList,
	//		//config.RefSpec(ref.Name()+":"+ref.Name()))
	//		config.RefSpec("refs/heads/master"+":"+"refs/heads/master"))
	//	return fmt.Errorf("References: %v", ref.Name())
	//})
	//fmt.Println("References: ", referenceList)

	//list, err := r.Remotes()
	//for _, r := range list {
	//	fmt.Println(r)
	//}

	// Get the ref list to be updated
	ref, err = r.Head()
	if err != nil {
		fmt.Println(err)
		// return err
	}
	fmt.Println("Head ref before PUSH command: " + ref.Name())
	var refList []config.RefSpec
	refList = append(refList,
		config.RefSpec(ref.Name()+":"+ref.Name()))

	//err = r.Push(&git.PushOptions{})
	err = r.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   refList,
		Auth: &http.BasicAuth{
			Username: "ashish246",
			Password: "Ashish1@github",
		},
	})
	if err != nil {
		fmt.Printf("Error Push: %v\n", err)
	}
}

func FetchGitFile() {
	//CheckArgs("<url>", "<directory>")
	//url := "https://github.service.anz/csp/opa-bundling-service"
	url := "https://github.com/ashish246/GolangGitExample.git"
	//directory := "."

	// fs doesn't have .git and all git stuff is in your Storer
	storage := memory.NewStorage()
	//fs contains all the files of the Git Repo
	fs := memfs.New()
	r, err := git.Clone(storage, fs, &git.CloneOptions{
		Auth: &http.BasicAuth{
			Username: "ashish246",
			Password: "Ashish1@github",
		},
		URL:      url,
		Progress: os.Stdout,
	})
	if err != nil {
		fmt.Errorf("Error: %s", err)
	}

	ref, err := r.Head()
	if err != nil {
		_ = fmt.Errorf("GIT repo HEAD not found: %v\n", err)
	}
	fmt.Println("Head ref before checkout: " + ref.Name())

	// Switch the branch to the configured one
	var worktree *git.Worktree
	worktree, err = r.Worktree()
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("release"),
		Create: true,
	})
	if err != nil {
		_ = fmt.Errorf("GIT Checkout command failed: %v\n", err)
	}

	// Check the current HEAD of the cloned repo. Must be the configured branch
	ref, err = r.Head()
	if err != nil {
		_ = fmt.Errorf("GIT repo HEAD not found: %v\n", err)
	}
	fmt.Println("Head after checkout: " + ref.Name())

	// Pull the new checked out branch for latest changes
	err = worktree.Pull(&git.PullOptions{RemoteName: "origin"})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		_ = fmt.Errorf("GIT Pull command failed: %v\n", err)
	}

	// fmt.Printf("Repo: %v\n", r)
	// Fetch a specific File from WITHIN THE FOLDER of the Git Repo
	f, err := fs.Open("opa-policy.rego")
	//f, err := fs.OpenFile("README.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Errorf("failed to fetch file info: %v", err)
	}

	// Print the latest commit that was just pulled
	//ref, err := r.Head()
	//fmt.Printf("head: %v\n", ref)
	//commit, err := r.CommitObject(ref.Hash())
	//fmt.Println(commit)

	reader := bufio.NewReader(f)
	yamlFile, err := ioutil.ReadAll(reader)
	//fmt.Printf("Read Text: %s\n", string(yamlFile))

	//yamlFile, err := ioutil.ReadFile(f.Name())
	//if err != nil {
	//	panic(err)
	//}

	err1 := os.Mkdir("tempOpa", 0755)
	if err1 != nil {
		fmt.Errorf("error: %v", err1)
	}
	//defer os.RemoveAll("tempOpa")

	err = ioutil.WriteFile("tempOpa/opa-policy.rego", yamlFile, 0644)
	if err != nil {
		panic(err)
	}
	//ParseYMLFile(yamlFile)
}

/*
https://stackoverflow.com/questions/28682439/go-parse-yaml-file
https://mholt.github.io/json-to-go/
http://convertjson.com/yaml-to-json.htm
https://github.com/go-yaml/yaml
*/

func ParseYMLFile() {
	// From the root of the package folder
	filename, _ := filepath.Abs("./opa-bundle-sample.json")
	//filename, _ := filepath.Abs("./opa-config.yml")
	yamlFile, err := ioutil.ReadFile(filename)

	if err != nil {
		panic(err)
	}
	//fmt.Print(yamlFile)

	//var config CloudBuild //
	var config LdapGroupEntitlements
	//var config OpaConfig
	//var err error

	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		panic(err)
	}

	//fmt.Printf("LDAP Group: %#v\n", config.LdapGroups[0].Name)
	//fmt.Printf("Service Name: %#v\nBundle Name: %#v\nDecision Log: %#v\nPolling Max Delay: %#v\n", config.Services[0].Name, config.Bundles[0]//.Name, config.DecisionLogs.Console, config.Bundles[0].Pooling.MaxDelaySeconds)

	// d, err := yaml.Marshal(&config)
	// if err != nil {
	// 	log.Fatalf("error: %v", err)
	// }

	//fmt.Printf("OpaConfig dump:\n%s\n\n", string(d))
	err = os.Mkdir("tempOpa", 0755)
	if err != nil {
		fmt.Errorf("error: %v", err)
	}
	// Delete the tempOpa/ folder
	defer os.RemoveAll("tempOpa")

	// Read the JSON file
	fileBytes, err := ioutil.ReadFile("opa-bundle-sample.json")
	if err != nil {
		log.Fatal(err)
	}

	err = ioutil.WriteFile("tempOpa/data.json", fileBytes, 0644)
	//err = ioutil.WriteFile("./opabundles/opa-bundle.json", opaBundleJSON, 0644)
	if err != nil {
		panic(err)
	}

	// Add REGO file
	FetchGitFile()

	// files, err := ioutil.ReadDir("tempOpa/")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// var paths []string
	// for _, f := range files {
	// 	fmt.Println(f.Name())
	// 	paths = append(paths, "tempOpa/"+f.Name())
	// }

	Tartar("../opa-bundling-service/opabundles/bundle-opapoc.tar.gz")

	oldLocation := "../opa-bundling-service/opabundles/bundle-opapoc.tar.gz"
	newLocation := "../opa-bundling-service/nginx/html/opapoc/bundle-opapoc.tar.gz"
	//err = os.Rename(oldLocation, newLocation)

	var srcfd *os.File
	var dstfd *os.File
	if srcfd, err = os.Open(oldLocation); err != nil {
		panic(err)
	}
	defer srcfd.Close()

	if dstfd, err = os.Create(newLocation); err != nil {
		panic(err)
	}
	//defer dstfd.Close()
	if _, err = io.Copy(dstfd, srcfd); err != nil {
		panic(err)
	}
	if err != nil {
		panic(err)
	}
}

func Tartar(tarName string) {

	files, err := ioutil.ReadDir("tempOpa/")
	if err != nil {
		log.Fatal(err)
	}
	//var paths []string
	for _, f := range files {
		fmt.Println(f.Name())
		//paths = append(paths, "tempOpa/"+f.Name())
	}

	tarWrite := func(data []os.FileInfo) error {
		tarFile, err := os.Create(tarName)
		if err != nil {
			log.Fatal(err)
		}
		defer tarFile.Close()
		tw := tar.NewWriter(tarFile)
		if strings.HasSuffix(tarName, ".gz") {
			gz := gzip.NewWriter(tarFile)
			defer gz.Close()
			tw = tar.NewWriter(gz)
		}
		defer tw.Close()
		for _, fileInfo := range data {
			hdr := &tar.Header{
				Name: fileInfo.Name(),
				Mode: 0600,
				Size: fileInfo.Size(),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			fileBytes, err := ioutil.ReadFile("tempOpa/" + fileInfo.Name())
			if err != nil {
				log.Fatal(err)
			}
			if _, err := tw.Write([]byte(fileBytes)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := tarWrite(files); err != nil {
		log.Fatal(err)
	}
}

// SearchUsers demonstrates how to use the search interface
func SearchUsers() {

	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", "localhost", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	err = conn.Bind("cn=admin,dc=globaltest,dc=anz,dc=com", "password")
	if err != nil {
		log.Fatal("Authenticating failed for user with given error: \n", err)
	}

	if err == nil {
		fmt.Printf("LDAP Connection made successfully for user %s. \n", "admin")
	}

	// Define list of attributes to be fetched
	attributes := []string{"creatorsName", "createTimestamp", "modifiersName", "modifyTimestamp", "entryDN", "entryUUID", "structuralObjectClass", "subschemaSubentry", "*"}

	searchRequest := ldap.NewSearchRequest(
		"cn=CAZ05,ou=Users,ou=AU,dc=globaltest,dc=anz,dc=com", // The base dn to search
		ldap.ScopeWholeSubtree,                                // Scope
		ldap.DerefAlways,                                      // DerefAliases
		0,                                                     // Size Limit
		0,                                                     // Time Limit
		false,                                                 // Types only flag
		"(&(objectClass=*)(modifyTimestamp>=20170925092902Z))", // The filter to apply
		attributes, // A list attributes to retrieve
		nil,        // Controls
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		log.Fatalf("Search query failed with the error: %v \n", err)
	}

	fmt.Printf("Total number of user entries found are ===============> %v\n", len(sr.Entries))
	for _, entry := range sr.Entries {
		fmt.Printf("ObjectClass: %v\n", entry.GetAttributeValues("objectClass"))
		fmt.Printf("CommonName: %v\n", entry.GetAttributeValue("cn"))
		fmt.Printf("sAMAccountName: %v\n", entry.GetAttributeValue("sAMAccountName"))
		fmt.Printf("Surname: %v\n", entry.GetAttributeValue("sn"))

		fmt.Printf("structuralObjectClass: %v\n", entry.GetAttributeValue("structuralObjectClass"))
		fmt.Printf("entryUUID: %v\n", entry.GetAttributeValue("entryUUID"))
		fmt.Printf("creatorsName: %v\n", entry.GetAttributeValue("creatorsName"))
		fmt.Printf("createTimestamp: %v\n", entry.GetAttributeValue("createTimestamp"))
		fmt.Printf("modifiersName: %v\n", entry.GetAttributeValue("modifiersName"))
		fmt.Printf("modifyTimestamp: %v\n", entry.GetAttributeValue("modifyTimestamp"))
		fmt.Printf("entryDN: %v\n", entry.GetAttributeValue("entryDN"))
		fmt.Printf("subschemaSubentry: %v\n", entry.GetAttributeValue("subschemaSubentry"))

		fmt.Printf("Description: %v\n", entry.GetAttributeValue("description"))
		fmt.Printf("SeeAlso: %v\n", entry.GetAttributeValue("seeAlso"))
		fmt.Printf("telephoneNumber: %v\n", entry.GetAttributeValue("telephoneNumber"))
	}
}

// SearchGroups demonstrates how to use the search interface
func SearchGroups(conn *ldap.Conn) {
	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", "localhost", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	err = conn.Bind("cn=admin,dc=globaltest,dc=anz,dc=com", "password")
	if err != nil {
		log.Fatal("Authenticating failed for user with given error: \n", err)
	}

	if err == nil {
		fmt.Printf("LDAP Connection made successfully for user %s. \n", "admin")
	}

	// Define list of attributes to be fetched
	attributes := []string{"creatorsName", "createTimestamp", "modifiersName", "modifyTimestamp", "entryDN", "entryUUID", "structuralObjectClass", "subschemaSubentry", "*"}

	searchRequest := ldap.NewSearchRequest(
		"cn=AU Digital BD Read,ou=Groups,ou=AU,dc=globaltest,dc=anz,dc=com", // The base dn to search
		ldap.ScopeWholeSubtree, // Scope
		ldap.DerefAlways,       // DerefAliases
		0,                      // Size Limit
		0,                      // Time Limit
		false,                  // Types only flag
		"(&(objectClass=*)(modifyTimestamp>=20170925092902Z))", // The filter to apply
		attributes, // A list attributes to retrieve
		nil,        // Controls
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		log.Fatalf("Search query failed with the error: %v \n", err)
	}

	fmt.Printf("Total number of group entries found are: ===============> %v\n", len(sr.Entries))

	for _, entry := range sr.Entries {
		fmt.Printf("Total number of users in the group %v entries found are: ===========> %v\n", entry.GetAttributeValue("cn"), len(entry.GetAttributeValues("uniqueMember")))

		fmt.Printf("ObjectClass: %v\n", entry.GetAttributeValues("objectClass"))
		fmt.Printf("CommonName: %v\n", entry.GetAttributeValue("cn"))

		fmt.Printf("structuralObjectClass: %v\n", entry.GetAttributeValue("structuralObjectClass"))
		fmt.Printf("entryUUID: %v\n", entry.GetAttributeValue("entryUUID"))
		fmt.Printf("creatorsName: %v\n", entry.GetAttributeValue("creatorsName"))
		fmt.Printf("createTimestamp: %v\n", entry.GetAttributeValue("createTimestamp"))
		fmt.Printf("modifiersName: %v\n", entry.GetAttributeValue("modifiersName"))
		fmt.Printf("modifyTimestamp: %v\n", entry.GetAttributeValue("modifyTimestamp"))
		fmt.Printf("entryDN: %v\n", entry.GetAttributeValue("entryDN"))
		fmt.Printf("subschemaSubentry: %v\n", entry.GetAttributeValue("subschemaSubentry"))

		fmt.Printf("businessCategory: %v\n", entry.GetAttributeValue("businessCategory"))
		fmt.Printf("OrganisationUnit: %v\n", entry.GetAttributeValue("ou"))
		fmt.Printf("Organisation: %v\n", entry.GetAttributeValue("o"))
		fmt.Printf("Description: %v\n", entry.GetAttributeValue("description"))
		fmt.Printf("Owner: %v\n", entry.GetAttributeValue("owner"))
		fmt.Printf("SeeAlso: %v\n", entry.GetAttributeValue("seeAlso"))

		var userNameStr string
		for _, user := range entry.GetAttributeValues("uniqueMember") {
			domainNames := strings.Split(user, ",")
			//fmt.Printf("Domain Names: %v \n", domainNames)
			userName := strings.Split(domainNames[0], "=")
			//fmt.Printf("User Names: %v \n", userName)
			userNameStr += userName[1]
			userNameStr += "|"
		}
		fmt.Printf("Users in the group %v are: ===========> %v\n", entry.GetAttributeValue("cn"), userNameStr)

		//fmt.Print("-----------------------------------------------------------------------------------------------------\n")
	}
}
