
<p align="center"><img src="docs/images/GoScout.png" alt="GoScout" width="20%"></p>

# GoScout

The GoScout is a tool with a UI for efficient and secure remote host management using key-based authentication. It's fully written in Go and requires no additional software installations.


[![Go](https://img.shields.io/badge/Go-1.23-blue)](https://golang.org)
[![Telegram](https://img.shields.io/badge/Telegram-Message-blue)](https://t.me/taradaidv)
[![Go](https://img.shields.io/badge/Go-100%25-brightgreen)](https://golang.org)
[![Code Status](https://img.shields.io/badge/Code%20Status-active-brightgreen.svg)](https://github.com/taradaidv/goscout/tree/main)

<br><p align="center"><img src="docs/images/screenshot.png" alt="GoScout"></p>

## Features
- **Security**: Utilizes SSH and exclusively certificates for reliable and secure connections.
- **Jump Hosts**: Supports connections through jump hosts for more complex network setups.
- **Minimalism**: Lightweight and fast to use, without unnecessary bloat.
- **Remembers state**: Keeps track of window size and last active tabs so you can continue working in your familiar environment.
- **UI**: [Fyne.io](https://fyne.io) toolkit is being used.
- **Hotkeys**: Text tweaked in the SSH config and file editor gets saved with the hotkeys CMD+S or CTRL+S.
- **Tabs**: Supports multiple tabs, allowing you to manage several sessions or files simultaneously.
- **Go**: Fully written in Go, ensuring high performance, reliability, and cross-platform compatibility.

## Build and Run

```
git clone https://github.com/taradaidv/goscout.git
cd goscout ; go build .
./goscout
```

## Persistent installation ~/go/bin and Run 

```
git clone https://github.com/taradaidv/goscout.git
cd goscout ; go install .
goscout
```

## TODO
There are lots of great things that could be added to this app.
Already planned is:

* Scroll-back
* Mouse actions
* Follow symlinks
* Integrate with IPFS
* Add Kubernetes support
* Download and upload folders and files.
* Add support for automatic detection of the host list on Windows
* And ...

---
This small utility is just the beginning of a larger project, and we need your help to maintain and expand the entire infrastructure. Join us in building something great!

<p align="center">
  <img src="docs/images/TON.png" alt="GoScout" width="30%">

  [TON Wallet address](https://ton.org)<br>
  UQDqFCrP01iTMfSFBHXFC-Q6S3CfsrCunVBy7DxWPYcxMsND
</p>