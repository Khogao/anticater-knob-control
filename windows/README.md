# Anticater Knob cho Windows

Bản Windows của Knob Console: núm Anticater VK-01 tự đổi chức năng theo ứng dụng đang mở.
Đây là mã viết mới bằng Go, không dùng chung mã với bản macOS ở thư mục gốc của repo.

> **Trạng thái (2026-11-14): đã biên dịch được, CHƯA TỪNG CHẠY trên Windows thật.**
> Toàn bộ mã được viết và biên dịch chéo trên macOS. Phần logic thuần (chọn hành động, khớp thiết bị,
> tên phím, so mốc thời gian, kích thước cấu trúc Win32) đã kiểm tra trên macOS. Mọi thứ đụng tới Windows
> (Raw Input, hook bàn phím, khay hệ thống, cửa sổ cài đặt) là phần cần thử đầu tiên. Xem mục 4 và mục 9.

Mục lục: 1. Chạy nhanh · 2. Build · 3. Dùng · 4. Checklist thử lần đầu · 5. Cấu hình · 6. Cách hoạt động ·
7. Bản đồ mã nguồn · 8. Dữ kiện đã xác nhận trên macOS · 9. Điểm chưa kiểm chứng và cách xử lý · 10. Việc nên làm tiếp ·
11. Khắc phục sự cố

---

## 1. Chạy nhanh

1. Chép cả thư mục `windows/` sang một nơi ghi được trên Windows, ví dụ `C:\Tools\AnticaterKnob\`.
   Đừng chạy từ `Program Files`: app ghi cấu hình và log ngay cạnh file exe.
2. Chạy `AnticaterKnob.exe` (bản biên dịch sẵn, 64-bit). SmartScreen có thể cảnh báo vì file không có chữ ký số:
   bấm "More info" → "Run anyway".
3. Biểu tượng xuất hiện ở khay hệ thống (có thể nằm trong mũi tên `^`). Lần chạy đầu app tạo `knob-config.json`.
4. Làm theo checklist ở mục 4.

`AnticaterKnob.exe` không nằm trong git (xem `.gitignore`); nếu thư mục không có file này thì build theo mục 2.

**Icon:** app đọc `AnticaterKnob.ico` nằm cạnh file exe lúc khởi động, dùng cho biểu tượng ở khay và cửa sổ cài đặt.
Không có file đó thì app dùng icon mặc định của Windows. Icon không được nhúng vào exe, nên file exe trong Explorer
vẫn hiện icon mặc định. Chép `AnticaterKnob.ico` đi cùng `AnticaterKnob.exe`.

File `.ico` được sinh từ ảnh vector `assets/logo.svg` ở thư mục gốc repo. Đổi màu logo: sửa thuộc tính `fill` của
nhóm `id="logo"` trong file SVG, hoặc truyền mã màu khi sinh icon (chạy trên macOS, từ thư mục gốc repo):

```bash
swift assets/make-icons.swift assets/logo.svg FF9124
```

Lệnh này ghi lại cả `windows/AnticaterKnob.ico` lẫn `assets/AppIcon.icns` (icon bản Mac). Trên Windows chưa có script
tương đương; có thể mở `logo.svg` bằng Inkscape rồi xuất ra `.ico`. Logo, file `.ico` và `.icns` đều không nằm trong git.

## 2. Build

Yêu cầu: **Go 1.21 trở lên**. Không cần thư viện ngoài, không cần cgo, không cần Visual Studio.

Cài Go trên Windows (một trong hai):

```bat
winget install GoLang.Go
```

hoặc tải bộ cài từ https://go.dev/dl/ . Mở cửa sổ lệnh mới sau khi cài, kiểm tra bằng `go version`.

Build:

```bat
build.bat
```

hoặc gõ tay trong PowerShell:

```powershell
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w -H=windowsgui" -o AnticaterKnob.exe .
```

- `-H=windowsgui` để app không mở cửa sổ console. Khi cần xem lỗi khởi động, build không có cờ này.
- Trên macOS / Linux: `./build.sh` (biên dịch chéo, kết quả giống hệt).
- Mọi file mã đều có hậu tố `_windows.go`, nên `go build` trên hệ khác mà không đặt `GOOS=windows` sẽ báo không có file.
- Phải thoát app đang chạy trước khi build lại, vì Windows khoá file exe đang chạy.

Kiểm tra tĩnh:

```bat
go vet ./...
```

`go vet` báo một dòng `possible misuse of unsafe.Pointer` ở `hookProc`. Đó là chỗ đọc cấu trúc do Windows truyền vào
callback của hook; cảnh báo này là bình thường.

## 3. Dùng

**Khay hệ thống**

- Chuột trái vào biểu tượng: mở cửa sổ cài đặt.
- Chuột phải: menu.

| Mục menu | Tác dụng |
|---|---|
| Gần nhất: … | Thao tác núm vừa nhận và hành động đã chạy (chỉ để xem) |
| Cài đặt… | Mở cửa sổ cài đặt |
| Tạm dừng | Ngừng xử lý; núm trở về hành vi gốc |
| Mở file cấu hình | Mở `knob-config.json` |
| Nạp lại cấu hình | Đọc lại file (app cũng tự nạp lại khi file thay đổi) |
| Thêm X vào cấu hình | Tạo hồ sơ cho app vừa dùng gần nhất, chép từ hồ sơ mặc định |
| Mở log | Mở `knob.log` |
| Báo cáo chẩn đoán | Ghi và mở `knob-diagnostic.txt` |
| Khởi động cùng Windows | Bật/tắt mục `AnticaterKnob` trong `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` |
| Thoát | Thoát app |

**Cửa sổ cài đặt**

- **Cột trái:** danh sách hồ sơ: "Mặc định (mọi app)" và từng app. "Thêm app…" liệt kê các app đang có cửa sổ;
  "Xoá" bỏ hồ sơ app đang chọn (không hỏi lại, nhưng chỉ có hiệu lực khi bấm Lưu).
- **Tên exe / Tiêu đề chứa:** khoá của hồ sơ. Để trống "Tiêu đề chứa" là áp dụng cho cả app; điền chữ thì chỉ áp dụng
  khi tiêu đề cửa sổ có chữ đó (ví dụ `chrome.exe` + `YouTube`).
- **Năm dòng thao tác:** Xoay trái, Xoay phải, Nhấn, Giữ + xoay trái, Giữ + xoay phải. Mỗi dòng có:
  - *Hành động*: loại hành động. "(theo mặc định)" nghĩa là hồ sơ app không ghi thao tác này, lấy từ hồ sơ mặc định.
  - *Phím / nút chuột / lệnh*: phím nối bằng dấu `+` (`ctrl+shift+tab`), hoặc `left`/`right`/`middle`, hoặc dòng lệnh.
  - *dy*, *dx*: số nấc cuộn. *Mỗi*: chỉ chạy mỗi N nấc.
  - *Chọn nhanh*: 33 hành động có sẵn; chọn một mục là dòng được điền đủ.
- **Mẫu nhanh:** điền một cặp trái/phải cho "Xoay" hoặc "Giữ + xoay".
- **Tín hiệu gần nhất:** cập nhật ngay khi núm gửi tín hiệu; dùng để biết núm có được nhận hay không.
- **Lưu:** ghi file và áp dụng. Đóng cửa sổ khi chưa lưu thì thay đổi bị bỏ, không có hỏi lại.

## 4. Checklist thử lần đầu

Làm theo thứ tự. Mỗi bước ghi kỳ vọng và việc cần làm nếu sai. Sau mỗi bước sai, chạy **Báo cáo chẩn đoán**
và giữ lại `knob-diagnostic.txt` cùng `knob.log`.

| # | Việc làm | Kỳ vọng | Nếu sai |
|---|---|---|---|
| 1 | Chạy exe | Có biểu tượng ở khay, có `knob-config.json` | Build bản không có `-H=windowsgui` rồi chạy từ console để xem lỗi. Hộp thoại "Raw Input registration failed" → mục 9.1 |
| 2 | Chuột phải vào biểu tượng | Menu tiếng Việt hiện đủ dấu | Menu vỡ chữ → lỗi chuyển UTF-16, xem `u16()` trong `winapi_windows.go` |
| 3 | Xoay núm ở desktop | Âm lượng Windows đổi (hồ sơ mặc định để Windows tự xử lý) | Núm chưa kết nối, không liên quan tới app |
| 4 | Xoay núm, rồi mở menu | "Gần nhất: rotate_right in … -> Volume up [global]" | Vẫn là "no knob input yet" → app không nhận tín hiệu, mục 9.1 và 9.2 |
| 5 | Chạy Báo cáo chẩn đoán | "Knob reports received" > 0; thiết bị núm ghi `knob=true` | `knob=false` → mục 9.3. Không có thiết bị nào → mục 9.1 |
| 6 | Mở Word, xoay núm | Zoom tài liệu, **âm lượng đứng yên** | Zoom được mà âm lượng vẫn đổi → mục 9.4. Không zoom → xem dòng "Gần nhất" có ghi `[WINWORD.EXE]` không |
| 7 | Mở Chrome, nhấn giữ + xoay | Chuyển tab | Không chuyển → mục 9.2 (mã `0x6F`/`0x70`). Trên laptop độ sáng đổi theo → mục 9.5 |
| 8 | Trong Word, bấm phím volume trên bàn phím thật | Âm lượng đổi, chậm khoảng 50 ms | Không đổi → mục 9.4 (phím bị coi nhầm là của núm) |
| 9 | Chuột trái vào biểu tượng | Cửa sổ cài đặt mở, chữ rõ, không bị cắt | Mục 9.6 |
| 10 | Trong cửa sổ: chọn một app, dùng Chọn nhanh, bấm Lưu | Dòng trạng thái "Đã lưu và áp dụng…", file JSON đổi | Hộp thoại lỗi ghi rõ dòng sai |
| 11 | Bật "Khởi động cùng Windows", đăng xuất, đăng nhập lại | App tự chạy | Kiểm tra giá trị registry ở mục 3 |

## 5. Cấu hình

File `knob-config.json` nằm cạnh file exe, tự tạo từ `default-config.json` (được nhúng vào exe) ở lần chạy đầu.
App tự nạp lại trong vòng 1 giây sau khi file thay đổi. File sai cú pháp thì app báo bằng bong bóng ở khay và giữ
cấu hình đang dùng.

```json
{
  "devices": ["05AC:022C", "514C:8850", "514C:4155"],
  "log": true,
  "global": {
    "rotate_left":        { "type": "passthrough" },
    "rotate_right":       { "type": "passthrough" },
    "press":              { "type": "passthrough" },
    "press_rotate_left":  { "type": "scroll", "dy": 1 },
    "press_rotate_right": { "type": "scroll", "dy": -1 }
  },
  "apps": {
    "WINWORD.EXE": {
      "rotate_left":  { "type": "scroll", "dy": -1, "keys": ["ctrl"] },
      "rotate_right": { "type": "scroll", "dy": 1,  "keys": ["ctrl"] }
    },
    "chrome.exe|YouTube": {
      "press_rotate_left":  { "type": "shortcut", "keys": ["left"] },
      "press_rotate_right": { "type": "shortcut", "keys": ["right"] }
    }
  }
}
```

**Khoá cấp cao nhất**

| Khoá | Ý nghĩa |
|---|---|
| `devices` | Danh sách `VID:PID` (hex, 4 chữ số mỗi phần) của núm. Chỉ tín hiệu từ các thiết bị này mới được xử lý |
| `log` | `true` thì ghi `knob.log` (tự xoá khi vượt 1 MB) |
| `global` | Hồ sơ mặc định |
| `apps` | Hồ sơ theo app |
| `_help` | Ghi chú, app bỏ qua |

**Thao tác:** `rotate_left`, `rotate_right`, `press`, `press_rotate_left`, `press_rotate_right`.

**Hành động**

| `type` | Tác dụng | Trường dùng |
|---|---|---|
| `passthrough` | Để Windows tự xử lý tín hiệu gốc của núm (volume, mute) | |
| `shortcut` | Giữ các phím bổ trợ, gõ lần lượt các phím còn lại | `keys` |
| `scroll` | Cuộn chuột; `keys` là phím bổ trợ giữ kèm (`ctrl` = zoom) | `dy`, `dx`, `keys` |
| `mouse` | Click tại vị trí con trỏ | `button`: `left` / `right` / `middle` |
| `shell` | Chạy lệnh qua `cmd /c`, không hiện cửa sổ | `command` |
| `noop` | Không làm gì, và vẫn chặn tín hiệu gốc | |

Trường chung: `description` (chữ hiển thị), `every` (chỉ chạy mỗi N nấc; bộ đếm tự về 0 sau 500 ms không xoay hoặc khi đổi thao tác).

`dy > 0` là cuộn lên, `dx > 0` là cuộn sang phải; mỗi đơn vị là một nấc chuột (120).

**Tên phím** (không phân biệt hoa thường)

- Bổ trợ: `ctrl` / `control`, `shift`, `alt` / `option` / `opt`, `win` / `super` / `meta`. `command` / `cmd` được coi là `ctrl`,
  để cấu hình từ bản macOS chép sang vẫn dùng được.
- Chữ `a`–`z`, số `0`–`9`, `f1`–`f24`.
- `enter` / `return`, `esc` / `escape`, `space`, `tab`, `backspace`, `delete` / `del`, `insert`, `home`, `end`, `pageup`, `pagedown`,
  `left`, `right`, `up`, `down`.
- Ký hiệu: `[` `]` `-` (`minus`) `=` (`plus`, `equal`) `,` `.` `/` `;` `'` `` ` `` `\`. Mã phím theo bàn phím US.
- Media: `volume_up`, `volume_down`, `mute`, `play_pause`, `next_track`, `previous_track`.

**Khớp app:** khoá trong `apps` là tên file exe (`WINWORD.EXE`, không phân biệt hoa thường), hoặc `exe|chữ` để chỉ khớp khi
tiêu đề cửa sổ chứa `chữ`. Với mỗi thao tác, app lấy mục có tiêu đề dài nhất khớp, rồi tới mục chỉ có tên exe, rồi tới `global`.
Một hồ sơ app chỉ cần ghi thao tác nó muốn đổi.

## 6. Cách hoạt động

Núm khai với hệ điều hành là một bàn phím và gửi phím đa phương tiện. Không có cách "chiếm" thiết bị ở mức ứng dụng,
nên app làm hai việc song song trên **một luồng duy nhất** (luồng chạy vòng lặp thông điệp):

```
Núm ──HID──► Windows ─┬─► Raw Input (WM_INPUT) ──► onRawInput ──► onKnobReport ──► perform(hành động)
                      │        chỉ thiết bị có VID:PID trong "devices"
                      │
                      └─► phím VK_VOLUME_UP / DOWN / MUTE ──► hookProc (WH_KEYBOARD_LL)
                                                              ├─ hành động là passthrough → cho qua
                                                              ├─ có tín hiệu núm cùng mốc thời gian → chặn
                                                              └─ chưa rõ → giữ 50 ms rồi quyết định (flushPending)
```

1. **Nhận tín hiệu núm.** `RegisterRawInputDevices` đăng ký trang Consumer Control (`0x0C`, usage `0x01`) với `RIDEV_INPUTSINK`
   để nhận cả khi app không ở phía trước. Mỗi `WM_INPUT` mang handle thiết bị; app lấy tên thiết bị và so `VID:PID`.
2. **Giải mã.** Gói tín hiệu có dạng `[report id, usage thấp, usage cao]`. Usage `0xE9`/`0xEA` = xoay phải/trái,
   `0xE2` hoặc `0xCD` = nhấn, `0x6F`/`0x70` = giữ + xoay phải/trái, `0` = nhả.
3. **Chọn hành động.** Lấy exe và tiêu đề của cửa sổ đang ở phía trước (lưu đệm 150 ms), tra cấu hình, chạy bằng `SendInput`.
4. **Chặn phím gốc.** Hook bàn phím cấp thấp thấy `VK_VOLUME_UP`, `VK_VOLUME_DOWN`, `VK_VOLUME_MUTE`, `VK_MEDIA_PLAY_PAUSE`.
   Phím do chính app gửi mang cờ `LLKHF_INJECTED` và luôn được cho qua.
5. **Phân biệt núm với bàn phím thật bằng mốc thời gian.** Mỗi tín hiệu núm được ghi mốc `GetMessageTime()`; mỗi phím trong hook
   có mốc `KBDLLHOOKSTRUCT.time`. Hai mốc cách nhau không quá 60 ms (`knobWindowMs`) thì phím là của núm và bị chặn.
   Nếu chưa có tín hiệu núm nào khớp, phím bị giữ lại 50 ms (`pendingHoldMs`); sau đó nếu vẫn không có thì nó là phím của
   bàn phím thật và được gửi lại.

So theo mốc thời gian của sự kiện chứ không theo lúc app xử lý là bài học từ bản macOS: ở đó so theo lúc nhận đã thả lọt
phím độ sáng, vì luồng chính chậm tới vài trăm mili giây khi đang chạy hành động.

Các hằng số nằm đầu `main_windows.go`: `knobWindowMs = 60`, `pendingHoldMs = 50`, `everyResetMs = 500`, `fgCacheMs = 150`.

## 7. Bản đồ mã nguồn

| File | Nội dung |
|---|---|
| `main_windows.go` | Điểm vào `main()`, vòng lặp thông điệp, `wndProc`. Nhận tín hiệu núm (`onRawInput`, `onKnobReport`), hook (`hookProc`, `flushPending`), so mốc thời gian (`noteKnobReport`, `knobReportedNear`), nạp cấu hình (`reloadConfig`, `housekeeping`), khay (`trayAdd`, `showMenu`, `balloon`), chẩn đoán (`writeDiagnostics`, `logf`) |
| `winapi_windows.go` | Khai báo hàm và cấu trúc Win32 qua `syscall.NewLazyDLL` (không cgo). Tiện ích: `foregroundApp`, `windowApp`, `rawDeviceName`, `sendKey`, `sendWheel`, `sendMouseButton`, `setAutostart`, `shellOpen` |
| `config_windows.go` | Kiểu `Config`, `Action`; `loadConfig`, `validate`; `matchesDevice` (khớp `VID:PID` trong đường dẫn thiết bị); `resolve` (chọn hành động theo app và tiêu đề) |
| `actions_windows.go` | Bảng tên phím → mã phím ảo (`namedKeys`, `lookupKey`), `perform` chạy hành động |
| `settings_windows.go` | Cửa sổ cài đặt bằng Win32 thuần: tạo control (`mk`, `combo`, `editBox`), nạp/ghi hồ sơ (`loadProfile`, `commitProfile`, `readRow`, `setRow`), `quickPicks`, `templates`, `saveSettings`, `settingsProc`; `initUI` và `enableVisualStyles` |
| `default-config.json` | Cấu hình mặc định, nhúng vào exe bằng `//go:embed` |
| `AnticaterKnob.ico` | Icon, đọc lúc chạy bởi `appIcon()` trong `winapi_windows.go`; sinh từ `../assets/logo.svg`, không nằm trong git |
| `build.bat`, `build.sh` | Script build cho Windows và cho macOS/Linux |

Lưu ý khi sửa:

- Mọi cấu trúc Win32 được khai báo tay. Kích thước đã kiểm tra trên amd64: `wndClassEx` 80, `msg` 48, `rawInputDevice` 16,
  `rawInputHeader` 24, `kbdLLHook` 24, `keyInput` / `mouseInput` 40, `notifyIconData` 976, `actCtx` 56. Chỉ hỗ trợ 64-bit.
- Mọi callback chạy trên luồng của `main()` (đã `runtime.LockOSThread`), nên các biến toàn cục không cần khoá.
  Đừng gọi hàm chậm trong `hookProc`: Windows gỡ hook nếu callback chậm quá ngưỡng (mặc định khoảng 300 ms).
- Thêm tên phím: sửa `namedKeys`. Thêm mục Chọn nhanh: sửa `quickPicks`. Thêm mẫu: sửa `templates`.

## 8. Dữ kiện đã xác nhận trên macOS

Cùng chiếc núm đã chạy đúng với bản macOS (thư mục gốc repo), dùng cùng cách tiếp cận. Những điều sau là đo thật:

- `VID:PID` là `05AC:022C` ở cả dây USB, dongle 2.4G và Bluetooth LE. Chip Jieli.
- Núm khai là bàn phím (usage page 1, usage 6) ở cả ba kiểu kết nối.
- Xoay gửi `0xE9` / `0xEA`; nhấn giữ + xoay gửi `0x6F` / `0x70`; mỗi lần kèm một gói nhả. Một nấc xoay = một lần nhấn-nhả.
- Tên thiết bị: qua USB/dongle là "USB Composite Device" (Jieli Technology); qua Bluetooth là "ANTICATER" (zhuhai_jieli).
- Trên macOS, phím hệ thống và tín hiệu núm tương ứng có mốc thời gian trùng nhau tới mili giây.
- Nhấn: bản ghi chú của tác giả gốc ghi `0xE2` (Mute); một đời núm khác gửi `0xCD` (Play/Pause). App nhận cả hai.

Các đời núm khác có ID khác (`514C:8850` qua dây, `514C:4155` qua dongle); cả ba ID đều có sẵn trong `devices`.

## 9. Điểm chưa kiểm chứng và cách xử lý

Mỗi mục: triệu chứng, cách xác nhận, hướng sửa. Bật log (`"log": true`) rồi đọc `knob.log`: mỗi tín hiệu núm, mỗi quyết định
của hook đều được ghi kèm mốc thời gian.

### 9.1 App không nhận được tín hiệu nào từ núm

- *Triệu chứng:* báo cáo chẩn đoán ghi "Knob reports received: 0" và không liệt kê thiết bị nào, dù âm lượng vẫn đổi khi xoay.
- *Kiểm tra:* bấm một phím media trên bàn phím thật. Nếu bàn phím đó hiện trong danh sách thiết bị thì Raw Input chạy,
  chỉ riêng núm không tới.
- *Hướng sửa:* núm có thể khai Consumer Control dưới một top-level collection khác. Trong `main()`, thử đăng ký theo cả trang:
  `rawInputDevice{UsagePage: 0x0C, Usage: 0, Flags: ridevInputSink | 0x20}` (`0x20` là `RIDEV_PAGEONLY`).
  Nếu vẫn không có, thử thêm trang `0x01` usage `0x06` (bàn phím) để xem núm gửi gì qua đường đó.

### 9.2 Nhận được tín hiệu nhưng không nhận diện được thao tác

- *Triệu chứng:* "Knob reports received" tăng, "Recognized gestures" đứng yên; log có dòng `(unmapped)`.
- *Kiểm tra:* xem các gói thô trong báo cáo. Trên macOS gói có dạng `03 e9 00`.
- *Hướng sửa:* nếu qua Bluetooth gói không có byte report id, hoặc usage chỉ 1 byte, sửa đoạn tính `usage` trong `onKnobReport`.
  Nếu mã khác hẳn, thêm vào `gestureForUsage`.

### 9.3 Thiết bị hiện ra nhưng `knob=false`

- *Triệu chứng:* báo cáo liệt kê thiết bị của núm với `knob=false`.
- *Hướng sửa:* nếu ID khác thì thêm `VID:PID` vào `devices`. Nếu ID đúng mà vẫn `false` thì đường dẫn thiết bị có dạng chưa lường
  (thường gặp với Bluetooth: `…VID&0205ac_PID&022c…`); sửa `matchesDevice` trong `config_windows.go`. Đường dẫn đầy đủ có trong log.

### 9.4 Âm lượng vẫn đổi ở app đã gán lại, hoặc phím volume của bàn phím thật bị nuốt

- *Kiểm tra:* trong báo cáo, "Native keys swallowed" phải tăng mỗi khi xoay núm ở app đã gán lại, và "Keys re-sent" phải tăng khi
  bấm phím volume trên bàn phím thật. Trong log, so `time=` ở dòng `report …` với `time=` ở dòng `hook …` của cùng một lần xoay.
- *Nếu hai mốc lệch nhau quá 60 ms:* tăng `knobWindowMs`. Nếu lệch một lượng cố định, bù vào trong `knobReportedNear`.
- *Nếu hook không ghi dòng nào:* Windows không sinh phím ảo cho tín hiệu của núm (âm lượng đổi qua đường khác). Khi đó cần chặn
  ở tầng khác, ví dụ đăng ký nhận `WM_APPCOMMAND` qua shell hook; phần này chưa có trong mã.
- *Nếu bong bóng báo "Keyboard hook failed":* `SetWindowsHookEx` thất bại; xem có phần mềm bảo mật chặn hook không.

### 9.5 Nhấn giữ + xoay vẫn đổi độ sáng màn hình

Hai mã `0x6F` / `0x70` không sinh phím ảo nên hook không thấy. Windows tự xử lý chúng cho màn hình tích hợp của laptop;
với màn hình ngoài thường không có tác dụng gì. Hiện không có cách chặn trong mã này. Nếu cần chỉnh độ sáng màn hình ngoài,
gán thao tác tới phím tắt của một công cụ DDC như Twinkle Tray hoặc Monitorian.

### 9.6 Cửa sổ cài đặt

Chưa từng được nhìn thấy trên Windows. Rủi ro:

- **Kiểu nút cũ:** `enableVisualStyles` lấy manifest giao diện từ `shell32.dll` (tài nguyên số 124). Nếu thất bại, control có kiểu
  Windows cũ nhưng vẫn dùng được. Cách chắc chắn hơn là nhúng manifest vào exe bằng file `.syso`.
- **DPI:** app gọi `SetProcessDPIAware` và nhân mọi toạ độ theo DPI hệ thống (`sx()`). Trên máy nhiều màn hình khác DPI có thể lệch.
- **Bố cục:** toạ độ đặt tay trong `openSettings`; cửa sổ rộng 1096 đơn vị. Chữ tiếng Việt dài có thể bị cắt ở nhãn.
- **Danh sách "Thêm app…":** dựa vào `EnumWindows`; app UWP có thể hiện là `ApplicationFrameHost.exe`.

### 9.7 App chạy quyền quản trị

`SendInput` từ app thường không tới được cửa sổ chạy quyền quản trị (UIPI), và có thể không đọc được tên exe của nó.
Nếu cần điều khiển các app đó, chạy `AnticaterKnob.exe` bằng quyền quản trị.

## 10. Việc nên làm tiếp

Theo thứ tự ưu tiên:

1. Chạy checklist mục 4 và sửa các lỗi lộ ra; đây là việc duy nhất thực sự chặn đường.
2. Đo độ lệch mốc thời gian giữa `WM_INPUT` và hook trên Windows, chỉnh `knobWindowMs` cho sát.
3. Thử cả ba kiểu kết nối (dây, dongle, Bluetooth), vì đường dẫn thiết bị và dạng gói có thể khác nhau.
4. Nhúng manifest và icon vào exe (file `.syso`), thay cho mẹo lấy manifest từ `shell32.dll` và việc đọc icon từ file rời.
5. Hỏi lại khi đóng cửa sổ cài đặt mà chưa lưu; thêm nút ghi phím tắt bằng cách bấm phím.
6. Thêm hành động độ sáng qua DDC nếu cần cho màn hình ngoài.
7. Ký số file exe để hết cảnh báo SmartScreen.

## 11. Khắc phục sự cố

| Triệu chứng | Nguyên nhân thường gặp |
|---|---|
| Không thấy biểu tượng ở khay | Nằm trong mũi tên `^`; hoặc app đã thoát vì lỗi khởi động (chạy bản build không có `-H=windowsgui` để xem) |
| "Already running" | Đã có một bản đang chạy; thoát từ menu khay |
| Sửa JSON không có tác dụng | File sai cú pháp: app báo bong bóng và giữ cấu hình cũ. Dùng "Nạp lại cấu hình" để xem lỗi |
| Hồ sơ app không áp dụng | Tên exe sai. Dùng "Thêm X vào cấu hình" hoặc "Thêm app…" để lấy đúng tên |
| Hành động chạy hai lần mỗi khấc | Đời núm gửi hai nấc mỗi khấc: đặt `"every": 2` (cột "Mỗi") |
| Không build lại được | App đang chạy nên file exe bị khoá |

File sinh ra khi chạy, đều nằm cạnh exe: `knob-config.json`, `knob.log`, `knob-diagnostic.txt`.
File app đọc nếu có: `AnticaterKnob.ico`.
