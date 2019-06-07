package main

import (
	"flag"
	"fmt"
	"github.com/muja/goconfig"
	"github.com/pkg/sftp"
	//"github.com/yookoala/realpath"
	"golang.org/x/crypto/ssh"
	"io"
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

	rlpath, err := filepath.Abs(path)

	if err != nil {
		fmt.Printf("get file: [%s] realpath failed: %s\n", path, err)
		return "", err
	}

	index := strings.LastIndex(rlpath, config["common.delim"])
	if index < 0 {
		//fmt.Printf("%s not found delim: %s\n", rlpath, config["common.delim"])
		return "", fmt.Errorf("%s not found delim: %s", rlpath, config["common.delim"])
	}

	rpath := fmt.Sprintf("%s/%s", config["common.remote"], rlpath[index:])

	//fmt.Printf("remote file: %s\n", rpath)

	return rpath, nil

}
func isFile(object string) (bool, error) {

	fdir, err := os.Open(object)
	if err != nil {
		fmt.Println(err)
		return false, err
	}
	defer fdir.Close()

	finfo, err := fdir.Stat()

	if err != nil {
		fmt.Println(err)
		return false, err
	}

	switch mode := finfo.Mode(); {

	case mode.IsDir():
		//fmt.Println("object is a directory")
		return false, nil
	case mode.IsRegular():
		//fmt.Println("object is a file")
		return true, nil
	}
	return false, nil
}
func push(scp *sftp.Client) {

	for i, _ := range targets {
		// Open a file
		srcFile, err := os.Open(targets[i])
		if err != nil {
			//log.Fatal(err)
			fmt.Printf("%s can not open, error: %s\n", targets[i], err)
			continue
		}
		defer srcFile.Close()

		finfo, err := srcFile.Stat()
		if err != nil {
			fmt.Printf("%s can not stat file or dir, error: %s\n", targets[i], err)
			continue
		}

		if !finfo.Mode().IsRegular() {
			fmt.Printf("%s is not regular file, transport file only\n", targets[i])
			continue
		}

		rpath, err := remotePath(targets[i])
		if err != nil {
			fmt.Println(err)
			continue
		}

		// create destination file
		dstFile, err := scp.Create(rpath)
		if err != nil {
			fmt.Printf("%s create file  failed, error: %s", rpath, err)
			continue
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			//log.Fatal(err)
			fmt.Printf("%s->%s scp file  failed, error: %s", targets[i], rpath, err)
			continue
		}
		fmt.Println("pushing:[local:]" + rpath + " to remote...")
	}
}

/*
func pull(scp *scplib.SCPClient) {

	for i, _ := range targets {
		// Open a file
		var fileCreated bool = false
		_, err := os.Stat(targets[i])
		if os.IsNotExist(err) {
			//maybe we should create a new FILE
			//_, err = os.Stat(filepath.Dir(targets[i]))
			file, err2 := os.Create(targets[i])
			if err2 != nil {
				fmt.Printf("%s  file not exists and create failed err: %s\n", targets[i], err2)
				continue
			}
			fileCreated = true
			file.Close()
		} else if err != nil {
				fmt.Printf("%s  open file  err: %s\n", targets[i], err)
				continue
		}
		file, err :=isFile(targets[i])
		if  err != nil || !file {
			fmt.Printf("%s not a file or open failed, err: %s\n", targets[i], err)
			continue
		}
		rpath, _ := remotePath(targets[i])   //how about a remote dir ?  we should check and skip

		fmt.Println("pulling: " + config["common.server"] +":"+ rpath + " to local...")
		if scp.GetFile([]string{rpath},  targets[i]) != nil {
			fmt.Println("Error while copying file ", err)
			if fileCreated == true {        //the filed create should be deleted
					if os.Remove(targets[i]) != nil {
						fmt.Println("remove the new  file error: ", err)
					}
			}
		}
	}

}
*/
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
	//scp := new(scplib.SCPClient)
	scp, err := sftp.NewClient(connection)
	//scp.Permission = false // copy permission with scp flag
	//scp.Connection = connection

	if mode == "push" {
		push(scp)
	} else {
		//pull(scp)
	}

}
