package main

import (
	"flag"
	"fmt"
	"github.com/muja/goconfig"
	"github.com/pkg/sftp"
	//"github.com/yookoala/realpath"
	"path/filepath"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"strings"
	"time"
)

var mode string
var targets []string
var config map[string]string
var homeDir string
var server string


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
            //if not the file, then the mode
            if _, err := os.Stat(flag.Args()[0]); err==nil {
                mode = config["common.mode"]
                targets = flag.Args()
            } else {
                mode = flag.Args()[0]
                server =  flag.Args()[1]
            }
		}
	}

	// quit if no target can be determined
	if len(targets) == 0 && (mode=="pull"||mode=="push")  {
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
		return "",  fmt.Errorf("%s not found delim: %s", rlpath, config["common.delim"])
	}

	rpath := fmt.Sprintf("%s/%s", config["common.remote"], rlpath[index:])

	//fmt.Printf("remote file: %s\n", rpath)

	return rpath, nil

}

func push(scp *sftp.Client) {

	for i, _ := range targets {
		// Open a file
		srcFile, err := os.Open(targets[i])
		if err != nil {
		   //log.Fatal(err)
		   fmt.Printf("%s can not open, error: %s\n",  targets[i], err);
			continue;
		}
		defer srcFile.Close()
		
		finfo, err := srcFile.Stat()
		 if err != nil {
		   fmt.Printf("%s can not stat file or dir, error: %s\n",  targets[i], err);
			continue;			 
		 }

		if !finfo.Mode().IsRegular() {
		   fmt.Printf("%s is not regular file, transport file only\n", targets[i]);
			continue;				
		}

		rpath, err := remotePath(targets[i])
		if err != nil {
			fmt.Println(err);
			continue;
		}

		// create destination file
		dstFile, err := scp.Create(rpath)
		if err != nil {
			fmt.Printf("%s create file  failed, error: %s", rpath, err)
			continue;
		}
		defer dstFile.Close()		
		
		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
		   //log.Fatal(err)
		   	fmt.Printf("%s->%s scp file  failed, error: %s", targets[i], rpath, err)
			continue;
		}
		fmt.Println("pushing:[local:]" + rpath + " to remote...")
	}
}

func pull(scp *sftp.Client) {
	for i, _ := range targets {
		
		// check dst file
		var newFile bool = false
		var  succ  bool = false
		var fileTrans string = ""
		finfo, err := os.Stat(targets[i])
		if err==nil && !finfo.Mode().IsRegular() {
		   fmt.Printf("%s is not regular file, transport file only\n", targets[i]);
			continue;				
		}
		if os.IsNotExist(err) {
			//maybe we should create a new FILE
			//_, err = os.Stat(filepath.Dir(targets[i]))
			newFile = true
			fileTrans = targets[i];
		} else {			
			fileTrans = targets[i] + ".scping"
		}
		//dstFile, err := os.Create(targets[i])
		dstFile, err := os.OpenFile(fileTrans, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, os.ModePerm)
		if err != nil {
			fmt.Printf("%s  file not exists and create failed err: %s\n", targets[i], err)
			continue
		}
		defer dstFile.Close()
		rpath, err := remotePath(targets[i])   
		if err!=nil {
			fmt.Println("get remote file path err: %s\n", err);
			//goto END
		}
		
		srcFile, err := scp.Open(rpath)
		if err != nil {
		   fmt.Printf("%s open remote file failed, error: %s\n", rpath, err);
		   //continue;
		   goto END
		}
		defer srcFile.Close()

		// copy source file to destination file
		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
		   fmt.Printf("%s copy file failed, error: %s\n", rpath, err);
		   //continue;
			 goto END
		}

		// flush in-memory copy
		err = dstFile.Sync()
		if err != nil {
		   fmt.Printf("%s copy file failed, error: %s\n", rpath, err);
		   //continue;
		   goto END
		}
		
		succ = true
		fmt.Println("pulling:[remote:]" + rpath + " to local...")

END:
		if succ == true {
			   if newFile==true {
				   continue
			   }
			if os.Remove(targets[i]) != nil {
				fmt.Println("remove the exist  file error: ", err)
			}
			if os.Rename(fileTrans, targets[i]) != nil {
				fmt.Printf("rename %s to %s failed, err: %s\n", fileTrans, targets[i], err)
			}
		} else {      //failed, remote the temp file
			if os.Remove(fileTrans) != nil {
				fmt.Printf("remove the tmp  file: %s error:  %s\n", fileTrans, err)
			}
		}
	}
	
}

func scpf(scp *sftp.Client) {
     
    // Open a file
    srcFile, err := os.Open(config[mode+".file"])
    if err != nil {
       //log.Fatal(err)
       fmt.Printf("%s can not open, error: %s\n", config[mode+".file"], err);
        return;
    }
    defer srcFile.Close()
		
    finfo, err := srcFile.Stat()
     if err != nil {
       fmt.Printf("%s can not stat file or dir, error: %s\n",  config[mode+".file"], err);
        return ;			 
     }

    if !finfo.Mode().IsRegular() {
       fmt.Printf("%s is not regular file, transport file only\n", config[mode+".file"]);
        return ;				
    }


    // create destination file
    dstFile, err := scp.Create(config[mode+".remote"])
    if err != nil {
        fmt.Printf("%s create file  failed, error: %s",  config[mode+".file"], err)
        return;
    }
    defer dstFile.Close()		
		
    _, err = io.Copy(dstFile, srcFile)
    if err != nil {
       //log.Fatal(err)
        fmt.Printf("%s->%s scp file  failed, error: %s", config[mode+".file"], config[mode+".remote"], err)
        return;
    }
    fmt.Println("pushing:[local:]" + mode + " to remote...")    
    
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
		Timeout:         2 * time.Second,
	}
    
    Server := config["common.server"]
    
    if mode != "push" && mode != "pull" {
        
        sconfig = &ssh.ClientConfig{
                User: config[mode+".user"],
                Auth: []ssh.AuthMethod{
                        ssh.Password(config[mode+".passwd"]),
                },
                HostKeyCallback: ssh.InsecureIgnoreHostKey(),
                Timeout:         12 * time.Second,
        }
        Server = server+":22"
    }
    
    
	// Create ssh connection
	connection, err := ssh.Dial("tcp", Server, sconfig)
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

    switch mode {
        case "push":
            push(scp)
        case "pull":
            pull(scp)
        default:
            scpf(scp)
    }


}
