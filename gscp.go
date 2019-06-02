package main

import (
	"flag"
	"fmt"
	"github.com/blacknon/go-scplib"
	"github.com/muja/goconfig"
	"github.com/yookoala/realpath"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

var mode string
var targets []string
var config map[string]string
var homeDir string

func readConfig() (map[string]string, error) {

	user, _ := user.Current()
	// don't forget to handle error!
	homeDir = user.HomeDir
	gconfig := filepath.Join(user.HomeDir, ".gscp")
	bytes, _ := ioutil.ReadFile(gconfig)

	config, _, err := goconfig.Parse(bytes)
	if err != nil {
		// Note: config is non-nil and contains successfully parsed values
		fmt.Printf("Error on line %d: %v.\n", err)
	}

	return config, err
	//fmt.Println(config["user.name"])
	//fmt.Println(config["user.email"])

}

func init() {

	flag.Parse()
	config, _ = readConfig()
	if flag.NArg() > 0 {
		if flag.Args()[0] == "push" || flag.Args()[0] == "pull" {
			mode = flag.Args()[0]
			for i := 1; i < flag.NArg(); i++ {
				targets = append(targets, flag.Args()[i])
			}
		} else {
			mode = config["common.mode"]
			targets = flag.Args()
		}
	}

	// quit if no target can be determined
	if len(targets) == 0 {
		fmt.Println("plz specify a file at least")
		os.Exit(1)
	}
}
func remotePath(path string) (string, error) {
	rlpath, err := realpath.Realpath(path)
	if err != nil {
		fmt.Printf("get file: [%s] realpath failed: %s\n", path, err)
		return "", err
	}

	index := strings.Index(rlpath, config["common.delim"])

	rpath := fmt.Sprintf("%s/%s", config["common.remote"], rlpath[index:])

	//fmt.Printf("remote file: %s\n", rpath)

	return rpath, nil

}

func push(scp *scplib.SCPClient) {

	for i, _ := range targets {
		// Open a file
		_, err := os.Stat(targets[i])
		if err != nil {
			fmt.Printf("open file: %s err: %s\n", targets[i], err)
			continue
		}
		rpath, _ := remotePath(targets[i])

		fmt.Println("pushing:" + rpath + " ...")
		// Close client connection after the file has been copied

		// Close the file after it has been copied

		// Finaly, copy the file over
		// Usage: CopyFile(fileReader, remotePath, permission)

		//err = client.CopyFile(f, rpath, "0655")
		//err = scp.PutFile([]string{"./passwd"}, "./passwd_scp")
		scp.PutFile([]string{targets[i]}, rpath)

		if err != nil {
			fmt.Println("Error while copying file ", err)
		}
	}
}

func pull(scp *scplib.SCPClient) {

	for i, _ := range targets {
		// Open a file
		_, err := os.Stat(targets[i])
		if err != nil {
			fmt.Printf("open file: %s err: %s\n", targets[i], err)
			continue
		}
		rpath, _ := remotePath(targets[i])
		
		if scp.GetFile([]string{rpath},  targets[i]) != nil {
			fmt.Println("Error while copying file ", err)
		}
	}
	
}

func main() {
	// Read Private key
	key, err := ioutil.ReadFile(homeDir + "/.ssh/id_rsa")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read private key: %v\n", err)
		os.Exit(1)
	}

	// Parse Private key
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse private key: %v\n", err)
		os.Exit(1)
	}

	// Create ssh client config
	sconfig := &ssh.ClientConfig{
		User: config["common.user"],
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         60 * time.Second,
	}

	// Create ssh connection
	connection, err := ssh.Dial("tcp", config["common.server"], sconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to dial: %s\n", err)
		os.Exit(1)
	}
	defer connection.Close()

	// Create scp client
	scp := new(scplib.SCPClient)
	scp.Permission = false // copy permission with scp flag
	scp.Connection = connection

	if mode == "push" {
		push(scp)
	} else {
		pull(scp)
	}

}
