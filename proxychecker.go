/*
    ProxyChecker - Used to get active proxy from public

    Git: https://github.com/wildy2832/ProxyChecker.git

    This project Licensed under MIT

    (C) Copyright 2023 - Wildy Sheverando [ Wildy2832 ]
*/

package proxychecker

import (
        "bufio"
        "fmt"
        "io/ioutil"
        "log"
        "net/http"
        "net/url"
        "os"
        "strings"
        "sync"
        "time"
)

type Proxy struct {
    Ip     string
    Port   string
    Status int // nilai status diubah menjadi integer
    Active bool
}

func checkProxy(proxy Proxy, wg *sync.WaitGroup, errChan chan error, activeFile *os.File) {
        defer wg.Done()

        if proxy.Status == 2 && proxy.Active {
                log.Printf("[%s:%s] Proxy ini telah di-scan", proxy.Ip, proxy.Port)
                return
        }

        if proxy.Status != 1 {
                log.Printf("[%s:%s] Ah taiklah proxy ini tidak valid", proxy.Ip, proxy.Port)
                proxy.Status = 0 // nilai status diubah menjadi 0 jika proxy tidak valid
                return
        }

        if proxy.Active {
                log.Printf("[%s:%s] Proxy ini sudah ada di active.txt", proxy.Ip, proxy.Port)
                return
        }

        transport := &http.Transport{}
        proxyURL, _ := url.Parse("http://" + proxy.Ip + ":" + proxy.Port)
        transport.Proxy = http.ProxyURL(proxyURL)

        client := &http.Client{
                Transport: transport,
                Timeout:   2 * time.Second,
        }

        resp, err := client.Get("https://raw.githubusercontent.com/wildy2832/testconnection/main/test.txt")
        if err != nil {
                resp, err = client.Get("http://raw.githubusercontent.com/wildy2832/testconnection/main/test.txt")
        }
        if err != nil {
                log.Printf("[%s:%s] Tidak dapat terhubung ke github.com", proxy.Ip, proxy.Port)
                proxy.Status = 0 // nilai status diubah menjadi 0 jika gagal terhubung ke github.com
                return
        }

        defer resp.Body.Close()

        body, err := ioutil.ReadAll(resp.Body)

        if err != nil {
                log.Printf("[%s:%s] Tidak dapat membaca respons dari github.com", proxy.Ip, proxy.Port)
                proxy.Status = 0 // nilai status diubah menjadi 0 jika gagal membaca respons dari github.com
                return
        }

        if strings.TrimSpace(string(body)) != "TEST KONEKSI BERHASIL" {
                log.Printf("[%s:%s] Respon tidak sesuai", proxy.Ip, proxy.Port)
                proxy.Status = 0 // nilai status diubah menjadi 0 jika respons tidak sesuai
                return
        }

        log.Printf("[%s:%s] Proxy ini valid", proxy.Ip, proxy.Port)
        proxy.Active = true
        if _, err := activeFile.WriteString(proxy.Ip + ":" + proxy.Port + "\n"); err != nil {
                errChan <- fmt.Errorf("Gagal menulis ke file -> %v", err)
        }
        proxy.Status = 2 // nilai status diubah menjadi 2 jika proxy valid dan belum dipindai sebelumnya
}

func main() {
    activeFile, err := os.OpenFile("active.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Println("| File active.txt tidak ditemukan ->", err)
        return
    }
    defer activeFile.Close()

    activeMap := make(map[string]bool)
    scanner := bufio.NewScanner(activeFile)
    for scanner.Scan() {
        activeMap[scanner.Text()] = true
    }

    var proxyList []Proxy

    urls := []string{
        "https://www.proxyscan.io/download?type=http",
        "https://www.proxyscan.io/download?type=https",
        "https://api.proxyscrape.com/?request=getproxies&proxytype=all&timeout=10000000&country=all&anonymity=all",
        "https://api.proxyscrape.com/?request=getproxies&proxytype=https&timeout=5000&country=all&ssl=all&anonymity=all",
        "https://api.proxyscrape.com/?request=getproxies&proxytype=http&timeout=5000&country=all&ssl=all&anonymity=all",
        "https://raw.githubusercontent.com/clarketm/proxy-list/master/proxy-list-raw.txt",
        "https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt",
        "https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks4.txt",
        "https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks5.txt",
        "https://raw.githubusercontent.com/mertguvencli/http-proxy-list/main/proxy-list/data.txt",
        "https://raw.githubusercontent.com/ShiftyTR/Proxy-List/master/proxy.txt",
        "https://raw.githubusercontent.com/proxy4parsing/proxy-list/main/http.txt",
        "https://raw.githubusercontent.com/caliphdev/Proxy-List/master/http.txt",
        "https://raw.githubusercontent.com/caliphdev/Proxy-List/master/socks5.txt",
    }

    var wg sync.WaitGroup
    errChan := make(chan error)

    defer func() {
        close(errChan)
        closeActiveFileErr := activeFile.Close()
        if closeActiveFileErr != nil {
            fmt.Println("Error while closing active file:", closeActiveFileErr)
        }
    }()

    // Looping utama
    for {
        // Looping untuk mengambil list proxy dari sumber yang berbeda
        for _, u := range urls {
            resp, err := http.Get(u)
            if err != nil {
                errChan <- fmt.Errorf("Gagal saat mengambil list proxy -> %v", err)
                continue
            }
            defer resp.Body.Close()

            body, err := ioutil.ReadAll(resp.Body)
            if err != nil {
                errChan <- fmt.Errorf("Gagal membaca response %s -> %v", u, err)
                continue
            }

            // Memisahkan proxy menjadi IP dan port
            scanner := bufio.NewScanner(strings.NewReader(string(body)))
            for scanner.Scan() {
                proxy := scanner.Text()
                if strings.Contains(proxy, "error code") {
                    log.Printf("Proxy error %s", proxy)
                    continue
                }

                proxySplit := strings.Split(proxy, ":")
                if len(proxySplit) < 2 {
                    continue
                }

                p := Proxy{Ip: proxySplit[0], Port: proxySplit[1], Status: 1} // nilai status diatur menjadi 1 jika proxy bisa dipindai
                _, exist := activeMap[p.Ip+p.Port]
                if exist {
                    log.Printf("[%s:%s] Proxy telah di-scan sebelumnya", p.Ip, p.Port)
                    p.Status = 2 // nilai status diatur menjadi 2 jika proxy telah dipindai sebelumnya
                    continue
                }

                proxyList = append(proxyList, p)
            }

            if err := scanner.Err(); err != nil {
                errChan <- fmt.Errorf("Gagal saat merequest data dari %s -> %v", u, err)
            }
        }

        // Looping untuk melakukan pengecekan proxy
        for i, p := range proxyList {
            wg.Add(1)
            go func(p Proxy, i int) {
                checkProxy(p, &wg, errChan, activeFile)
                proxyList[i] = p
            }(p, i)
        }

        wg.Wait()

        // Filterisasi proxy yang duplikat dan invalid
        activeMap = make(map[string]bool)
        var filteredList []Proxy
        for _, p := range proxyList {
            if p.Status == 2 && p.Active {
                activeMap[p.Ip+p.Port] = true
            } else if p.Status == 1 && !p.Active {
                filteredList = append(filteredList, p)
            }
        }

        proxyList = filteredList

        // Pause for 10 seconds before requesting again
        time.Sleep(10 * time.Second)
    }

    go func() {
        for err := range errChan {
            log.Println("Terjadi kesalahan ->", err)
        }
    }()
}
