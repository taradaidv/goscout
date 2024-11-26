
<p align="center"><img src="docs/images/GoScout.png" alt="GoScout" width="20%"></p>

# GoScout

The GoScout is a UI tool for efficient and secure remote host management using ssh. It's fully written in Go and requires no additional software installations.


[![Go](https://img.shields.io/badge/Go-1.23-blue)](https://golang.org)
[![Telegram](https://img.shields.io/badge/Telegram-Message-blue)](https://t.me/taradaidv)
[![Go](https://img.shields.io/badge/Go-100%25-brightgreen)](https://golang.org)
[![Code Status](https://img.shields.io/badge/Code%20Status-active-brightgreen.svg)](https://github.com/taradaidv/goscout/tree/main)

<p align="center"><img src="docs/images/screenshot.png" alt="GoScout"></p>

## Features
- **Go**: Fully written in Go, ensuring high performance, reliability, and cross-platform compatibility.
- **Hotkeys**: Text tweaked in the SSH config and file editor gets saved with the hotkeys CMD+S or CTRL+S.
- **Jump Hosts**: Supports connections through jump hosts for more complex network setups.
- **Minimalism**: Lightweight and fast to use, without unnecessary bloat.
- **Remembers state**: Keeps track of window size and last active tabs so you can continue working in your familiar environment.
- **Security**: Uses SSH and SFTP with private keys for secure and reliable connections.
- **Tabs**: Supports multiple tabs, allowing you to manage several sessions or files simultaneously.
- **UI**: [Fyne.io](https://fyne.io) toolkit is being used.

## Build and Run

```
git clone https://github.com/taradaidv/goscout.git
cd goscout && go build . && ./goscout
```

## Persistent installation ~/go/bin and Run 

```
git clone https://github.com/taradaidv/goscout.git
cd goscout && go install . && goscout
```

## TODO
There are lots of great things that could be added to this app.
Already planned is:

*Legend*  
⭕️ *abandoned*  
🟢 *implemented*  
⚪️ *developing* 

|**Planned Feature**| **Progress**|
|-|-|
|Add Kubernetes support|-|
|Add support for detection of the host list on Windows|-|
|Connection process output in the app window|🟢|
|Follow symlinks|⚪️|
|Integrate with IPFS|-|
|Mouse actions|🟢|
|Password input support for *ssh* and *sftp*|🟢|
|Scroll-back|⚪️|
|Sync files and folders via native OS file manager|⚪️|
|...|...|


## Support the project
This small utility is just the beginning of a larger project, and we need your help to maintain and expand the entire infrastructure. Join us in building something great!

<p align="center">
  <img src="docs/images/TON.png" alt="GoScout" width="30%">

  [TON Wallet address](https://ton.org)  
  UQDqFCrP01iTMfSFBHXFC-Q6S3CfsrCunVBy7DxWPYcxMsND
</p>