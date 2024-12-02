package HandlePacket

import (
	"fmt"
	"main/MessagePack"
	"main/util/setchannel"
	"main/util/setchannel/ptyopt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/togettoyou/wsc"
	"golang.org/x/term"
)

func PtyCmd(Controler_HWID string, unmsgpack MessagePack.MsgPack, Connection *wsc.Wsc) {
	var err error
	// 初始化Pty终端数据通道
	ptyDataChan, exist := setchannel.GetPtyDataChan(Controler_HWID)
	if !exist {
		ptyDataChan = make(chan interface{})
		setchannel.AddPtyDataChan(Controler_HWID, ptyDataChan)
	}
	defer setchannel.DeletePtyDataChan(Controler_HWID)
	// Start the command with a pty.
	ptmx := ptyopt.InitPtmx()

	defer func() { _ = ptmx.Close() }() // Best effort.

	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				//fmt.Println("error resizing pty: " + err.Error())
			}
		}
	}()
	ch <- syscall.SIGWINCH                        // Initial resize.
	defer func() { signal.Stop(ch); close(ch) }() // Cleanup signals when done.

	// Set stdin in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

	// first read
	buffer := make([]byte, 4096)
	_, err = ptmx.Read(buffer)
	if err != nil {
		return
	}
	fmt.Println("Raw Output:", string(buffer))
	time.Sleep(300 * time.Millisecond)
	ptyopt.RetPtyResult(buffer, Controler_HWID, unmsgpack, Connection)

	func() {
		for {
			select {
			//case <-time.After(60 * time.Second):
			//	log.Println("exit pty")
			//	return
			case data := <-ptyDataChan:
				// write
				buf, ok := data.([]byte)
				if string(buf) == "exit\n" {
					//log.Println("exit pty")
					return
				}
				if !ok {
					return
				}
				_, err = ptmx.Write(buf)
				if err != nil {
					break
				}
				time.Sleep(200 * time.Millisecond)
				// read
				buffer = make([]byte, 4096)
				_, err = ptmx.Read(buffer)
				if err != nil {
					return
				}
				ptyopt.RetPtyResult(buffer, "", unmsgpack, Connection)
				time.Sleep(300 * time.Millisecond)
			}
		}
	}()

}
