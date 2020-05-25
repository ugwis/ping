package main

import (
    "errors"
    "flag"
    "fmt"
    "log"
    "net"
    "os"
    "time"

    "golang.org/x/net/icmp"
    "golang.org/x/net/ipv4"
)

func Ping(proto string, host string, seq int, data_size int, timeout time.Duration) (string, error) {
    conn, err := net.DialTimeout(proto+":icmp", host, timeout)
    if err != nil {
        return "", err
    }
    defer conn.Close()
    conn.SetWriteDeadline(time.Now().Add(timeout))
    wm := icmp.Message{
        Type: ipv4.ICMPTypeEcho,
        Code: 0,
        Body: &icmp.Echo{
            ID: os.Getpid() & 0xffff, Seq: seq,
            Data: []byte("HELLO-R-U-THERE"),
        },
    }
    wb, err := wm.Marshal(nil)
    if err != nil {
        log.Fatalf("Marshal: %v", err)
    }

    size, err := conn.Write(wb)
    if err != nil {
        return "", err
    }
    if size != len(wb) {
        return "", errors.New("send ping data err")
    }
    begin := time.Now()

    c := make(chan func() (string, error))
    go func(c chan func() (string, error)) {
        for time.Now().Sub(begin) < timeout{
            data := make([]byte, 20+size) // ping recv back
            //conn.SetReadDeadline(time.Now().Add(timeout))
            _, err := conn.Read(data)
            if err != nil {
                c <- (func() (string, error) { return "", err })
                return
            }
            header, err := ipv4.ParseHeader(data)
            if err != nil {
                c <- (func() (string, error) { return "", err })
                return
            }
            var msg *icmp.Message
            msg, err = icmp.ParseMessage(1, data[header.Len: header.TotalLen])
            if err != nil {
                c <- (func() (string, error) { return "", err })
                return
            }
            switch msg.Type {
            case ipv4.ICMPTypeEcho:
                continue
            case ipv4.ICMPTypeEchoReply:
                msg.Body.Marshal(1)
                if echo, ok := msg.Body.(*icmp.Echo); !ok {
                    c <- (func() (string, error) { return "", errors.New("ping recv err data") })
                    return
                }else{
                    end := time.Now()
                    msg := fmt.Sprintf("64 bytes from %v: icmp_seq=%d ttl=%d time=%.3f ms", header.Src, echo.Seq, header.TTL, (end.Sub(begin)).Seconds()*1000)
                    c <- (func() (string, error) { return msg, nil })
                    return
                }
            default:
                continue
            }
        }
        c <- (func() (string, error) { return "", nil })
        return
    }(c)
    select {
    case rcv := <-c:
        msg, err := rcv()
        return msg, err
    case <-time.After(timeout):
        return "", errors.New(fmt.Sprintf("Request timeout for icmp_seq %d", seq))
    }
}

func main(){
    var sleep time.Duration
    var timeout time.Duration
    var size int
    var ipv6 bool

    flag.DurationVar(&sleep, "i", 1000*time.Millisecond, "interval")
    flag.DurationVar(&timeout, "t", 500*time.Millisecond, "timeout")
    flag.IntVar(&size, "s", 64, "size")
    flag.BoolVar(&ipv6, "6", false, "ip6")
    flag.Parse()
    host := flag.Arg(0)

    proto := "ip4"
    if ipv6 {
        proto = "ip6"
    }


    for i := 0 ;; i++{
        msg, err := Ping(proto, host, i, size, 1 * time.Second)
        if err != nil {
            //log.Println(err)
            fmt.Println(err)
        } else {
            fmt.Println(msg)
        }
        time.Sleep(sleep)
    }
}
