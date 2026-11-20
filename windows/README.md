# Anticater Knob cho Windows

Bản Windows của Knob Console: núm Anticater VK-01 tự đổi chức năng theo ứng dụng đang mở.
Đây là mã viết mới bằng Go, không dùng chung mã với bản macOS ở thư mục gốc của repo.

> **Trạng thái (2026-11-14): đã biên dịch được, CHƯA TỪNG CHẠY trên Windows thật.**
> Toàn bộ mã được viết và biên dịch chéo trên macOS. Phần logic thuần (chọn hành động, khớp thiết bị,
> tên phím, so mốc thời gian, kích thước cấu trúc Win32) đã kiểm tra trên macOS. Mọi thứ đụng tới Windows
> (Raw Input, hook bàn phím, khay hệ thống, cửa sổ cài đặt) là phần cần thử đầu tiên. Xem mục 4 và mục 9.

Mục lục: 1. Chạy nhanh · 2. Build · 3. Dùng · 4. Checklist thử lần đầu · 5. Cấu hình · 6. Cách hoạt động ·
7. Bản đồ mã nguồn · 8. Dữ kiện đã xác nhận trên macOS · 9. Điểm chưa kiểm chứng và cách xử lý · 10. Việc nên làm tiếp ·
11. Khắc phục sự cố · 12. Tài nguyên kèm theo

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
- `go.mod` nằm ở thư mục gốc repo (module `anticater-knob`); app Windows là gói `./windows`, trang cài đặt dùng chung là gói `./ui`.
  Các script build vẫn chạy từ thư mục `windows/` như trước.
- Mọi file mã trong `windows/` đều có hậu tố `_windows.go`, nên `go build` trên hệ khác mà không đặt `GOOS=windows` sẽ báo không có file.
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

**Ngôn ngữ**

Giao diện có hai ngôn ngữ: tiếng Việt và tiếng Anh. Lần chạy đầu app theo ngôn ngữ hiển thị của Windows
(Windows tiếng Việt thì tiếng Việt, còn lại tiếng Anh). Đổi bằng một trong ba cách:

- Menu ở khay: mục "Language: English" (khi đang ở tiếng Việt) hoặc "Ngôn ngữ: Tiếng Việt" (khi đang ở tiếng Anh).
- Cửa sổ cài đặt: menu "Công cụ" → "Language: English" / "Ngôn ngữ: Tiếng Việt". Trang đổi ngay, các thay đổi chưa lưu được giữ.
- File cấu hình: `"language": "vi"` hoặc `"en"`; bỏ khoá này để theo Windows.

Lựa chọn được ghi vào `knob-config.json` ngay khi đổi. Menu, cửa sổ cài đặt, bảng Chọn nhanh, mẫu nhanh và các thông báo
đều đổi theo. Dòng "Gần nhất / Last", file log và báo cáo chẩn đoán luôn bằng tiếng Anh. Tên menu trong tài liệu này ghi
theo bản tiếng Việt.

**Cửa sổ cài đặt**

Cửa sổ cài đặt là một trang web nhúng sẵn trong file exe (gói `ui/`, dùng chung cho bản Windows và bản Linux sau này),
bố cục theo app macOS. App mở một máy chủ chỉ nghe trên `127.0.0.1` (cổng 47815, bận thì lấy cổng trống khác) rồi mở
trang trong cửa sổ Edge chế độ app: không tab, không thanh địa chỉ. Máy không có Edge thì dùng Chrome; không có cả hai
thì trang mở thành một tab của trình duyệt mặc định. Chạy `AnticaterKnob.exe` lần nữa khi app đang chạy cũng mở cửa sổ này.

- **Thanh bên:** "Mặc định chung", các app đã có hồ sơ, và "App đang chạy" (bấm một app để tạo hồ sơ cho nó). Bên dưới
  có ô tìm app, nút Làm mới, ô nhập tên exe bằng tay, "Thêm app hiện tại" và "Xoá app đang chọn".
- **Tên exe / Tiêu đề cửa sổ chứa:** khoá của hồ sơ app. Để trống tiêu đề là áp dụng cho cả app; điền chữ thì chỉ áp dụng
  khi tiêu đề cửa sổ có chữ đó (ví dụ `chrome.exe` + `YouTube`).
- **Bảng trạng thái:** đang lắng nghe hay tạm dừng, núm đã gửi tín hiệu chưa, app phía trước, hồ sơ đang áp dụng cho app đó,
  hook chặn phím gốc, tín hiệu gần nhất. "App phía trước" là app dùng gần nhất, không tính chính cửa sổ cài đặt.
- **Theo dõi tín hiệu:** bộ đếm tín hiệu thô, số thao tác nhận diện được và bộ đếm riêng cho từng thao tác; ô của thao tác
  vừa nhận sẽ nháy sáng.
- **Mẫu nhanh:** điền một cặp trái/phải cho "Xoay" hoặc "Giữ + xoay", cho hồ sơ đang chọn hoặc tạo luôn hồ sơ cho app phía trước.
- **Công cụ:** khôi phục âm lượng cho hồ sơ mặc định, xoá bộ đếm, báo cáo chẩn đoán, mở file cấu hình, mở log,
  khởi động cùng hệ thống, đổi ngôn ngữ.
- **Năm thẻ thao tác:** Xoay trái, Xoay phải, Nhấn, Giữ + xoay trái, Giữ + xoay phải. Mỗi thẻ có:
  - *Hành động*: loại hành động. "(theo mặc định)" nghĩa là hồ sơ app không ghi thao tác này, lấy từ hồ sơ mặc định.
  - *Chọn nhanh*: 35 hành động có sẵn, chia nhóm; chọn một mục là thẻ được điền đủ.
  - Các ô theo loại: phím nối bằng dấu `+` (`ctrl+shift+tab`) kèm nút "Ghi phím…" để bấm tổ hợp thay vì gõ tên;
    số nấc cuộn dọc/ngang và phím giữ kèm; nút chuột; dòng lệnh. *Mỗi N nấc*: chỉ chạy mỗi N nấc. *Ghi chú*: mô tả tuỳ ý.
- **Lưu cấu hình:** kiểm tra, ghi file và áp dụng; nút đổi màu khi có thay đổi chưa lưu. Lỗi kiểm tra hiện ở thanh dưới cùng.
  Đóng cửa sổ khi chưa lưu thì thay đổi bị bỏ, không có hỏi lại. **Nạp lại** đọc lại file, bỏ phần chưa lưu.

API của trang chỉ nhận yêu cầu mang mã truy cập sinh ngẫu nhiên mỗi lần app chạy (truyền qua phần `#` của địa chỉ) và
đúng tên máy chủ `127.0.0.1:<cổng>`, vì cấu hình có thể chứa lệnh shell. Cửa sổ còn mở từ lần chạy trước sẽ báo
"Phiên này đã cũ"; mở lại từ biểu tượng ở khay.

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
| 12 | Đổi ngôn ngữ từ menu khay, rồi từ menu Công cụ trong cửa sổ cài đặt | Menu và cửa sổ đổi ngôn ngữ; thay đổi chưa lưu vẫn còn; `knob-config.json` có khoá `language` | Trang không đổi → xem `setLanguage` trong `ui/web/app.js` và `switchLanguage` |

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
| `language` | `"vi"` hoặc `"en"`; không có thì theo ngôn ngữ hiển thị của Windows |
| `global` | Hồ sơ mặc định |
| `apps` | Hồ sơ theo app |
| `led` | Màu đèn của núm theo app (xem "Đèn của núm" ngay dưới) |
| `_help` | Ghi chú, app bỏ qua |

**Đèn của núm** (`led_windows.go`, `knob-led.ps1`): đèn đổi màu khi app phía trước đổi.

```json
"led": { "brightness": 45, "auto": true,
         "apps": { "desktop": "FFFFFF", "explorer.exe": "FFC800", "msedge.exe": "00A7E1", "chrome.exe": "FF0000" } }
```

- Khoá trong `apps` khớp như khoá hồ sơ (`exe` hoặc `exe|chữ`). `desktop` là màn hình nền của Windows; khi đó `explorer.exe`
  chỉ còn là cửa sổ File Explorer. Thanh tác vụ và menu Start không đổi màu.
- Màu là `RRGGBB` hoặc `off`. `brightness` từ 1 đến 100.
- `auto: true`: app chưa có màu thì lấy màu đặc trưng của app quen (`brandColours`), còn lại một màu cố định sinh từ tên exe.
  Không có `auto` thì dùng `global`; `global` trống thì giữ nguyên màu.
- Sửa trong cửa sổ cài đặt: thẻ "Đèn của núm" ở đầu mỗi hồ sơ. Ở "Mặc định chung" là màu màn hình nền, công tắc tự động và độ sáng.
- Cách gửi: lệnh của phần mềm ANTICATER chính hãng (đọc từ mã của nó, 21/11/2026), gói 64 byte `03 FD 00 06 R G B độ_sáng bật 00…`,
  ghi không chờ trả lời vào đặc tính `AE41` của dịch vụ Bluetooth `AE40` trên núm. Chỉ có một màu cho cả núm, không đặt riêng từng đèn.
  App chạy `knob-led.ps1` bằng Windows PowerShell một lần, ẩn, rồi gửi từng dòng màu qua đầu vào chuẩn. Đã thử với núm thật qua Bluetooth.
- Phần mềm của hãng đang mở thì nó giữ dịch vụ `AE40` và lệnh màu bị từ chối (log ghi `SharingViolation`). Đóng phần mềm đó.

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
   Phím do chính app gửi mang dấu riêng trong `dwExtraInfo` (`ownInputMark`) và luôn được cho qua. Không dùng cờ
   `LLKHF_INJECTED` để nhận biết: với núm Bluetooth LE, chính phím volume của núm cũng mang cờ này.
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
| `brightness_windows.go` | Độ sáng: màn hình ngoài qua DDC/CI, màn hình tích hợp của laptop qua `\\.\LCD` (`adjustPanel`). `adjustBrightness` xếp yêu cầu, luồng riêng `brightnessWorker` gửi lệnh |
| `ui_windows.go` | Cầu nối giữa lõi và trang cài đặt: `uiBackend` (trạng thái, đọc/ghi cấu hình, danh sách app, lệnh), `onMain` đưa yêu cầu từ luồng HTTP về luồng chính, `openUI` mở hoặc đưa cửa sổ lên trước, `runningApps` |
| `lang_windows.go` | Hai ngôn ngữ cho menu khay và thông báo: `L("tiếng Việt", "English")` chọn chuỗi theo ngôn ngữ hiện tại; `setLanguage` |
| `../ui/ui.go` | Gói dùng chung, không phụ thuộc hệ điều hành: máy chủ HTTP nội bộ, kiểm tra mã truy cập, giao diện `Backend` mà mỗi nền tảng tự cài |
| `../ui/web/` | Trang cài đặt: `index.html`, `app.css`, `app.js` (bố cục, mẫu nhanh, bảng Chọn nhanh, ghi phím, hai ngôn ngữ) |
| `../ui/uidev/` | `go run ./ui/uidev` chạy trang với dữ liệu giả lập, để sửa giao diện trên máy bất kỳ mà không cần núm |
| `default-config.json` | Cấu hình mặc định, nhúng vào exe bằng `//go:embed` |
| `AnticaterKnob.ico` | Icon, đọc lúc chạy bởi `appIcon()` trong `winapi_windows.go`; sinh từ `../assets/logo.svg`, không nằm trong git |
| `build.bat`, `build.sh` | Script build cho Windows và cho macOS/Linux |

Lưu ý khi sửa:

- Mọi cấu trúc Win32 được khai báo tay. Kích thước đã kiểm tra trên amd64: `wndClassEx` 80, `msg` 48, `rawInputDevice` 16,
  `rawInputHeader` 24, `kbdLLHook` 24, `keyInput` / `mouseInput` 40, `notifyIconData` 976, `actCtx` 56. Chỉ hỗ trợ 64-bit.
- Mọi callback chạy trên luồng của `main()` (đã `runtime.LockOSThread`), nên các biến toàn cục không cần khoá.
  Đừng gọi hàm chậm trong `hookProc`: Windows gỡ hook nếu callback chậm quá ngưỡng (mặc định khoảng 300 ms).
- Thêm tên phím: sửa `namedKeys`. Thêm mục Chọn nhanh: sửa `quickPicks`. Thêm mẫu: sửa `templates`.
- Mọi chuỗi hiện ra cho người dùng phải viết dạng `L("tiếng Việt", "English")`, hoặc `text{"…", "…"}` trong các bảng.
  Nhãn tiếng Anh thường dài hơn; kiểm tra độ rộng control trong `openSettings` khi thêm.

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

Hai mã `0x6F` / `0x70` không sinh phím ảo nên hook không thấy (đã kiểm tra bằng log: không có phím nào đi kèm tín hiệu
giữ + xoay). Phím âm lượng thì khác: dịch vụ `hidserv` đổi chúng thành phím qua `SendInput`, nên hook chặn được.

Mã độ sáng do dịch vụ Power của Windows tự đọc thẳng từ thiết bị HID (`umpoext.dll`, "HID brightness device"), song song
với app. Vì vậy mỗi lần giữ + xoay, Windows hiện biểu tượng độ sáng dù thao tác đã được gán việc khác, và màn hình tích hợp
của laptop đổi độ sáng thật. Từ một app thường không chặn được việc này.

**Chính sách `BrightnessButtonDisabled` không dùng được trên Windows thường** (đã đọc mã `umpoext.dll` 10.0.26100.8875,
21/11/2026). Mỗi lần nhận mã độ sáng, `umpoext.dll` có xem cờ "bỏ qua theo chính sách", nhưng nó chỉ đọc giá trị
`PolicyManager\Current\Device\MixedReality\BrightnessButtonDisabled` khi `RtlGetDeviceFamilyInfoEnum` trả về 10 (HoloLens).
Trên máy tính thường (giá trị 3) cờ luôn tắt, đổi registry không có tác dụng.

**Cách chặn: đổi tín hiệu ngay trong núm.** Phần mềm cấu hình của hãng (bản Qt 6 có `ble_helper.exe`, ghi cấu hình qua
Bluetooth, nhận đời núm `05AC:022C`) cho gán phím cho từng thao tác. Gán:

| Thao tác | Gán thành |
|---|---|
| Giữ + xoay phải | F13 |
| Giữ + xoay trái | Shift + F13 |

App nhận F13 trong hook bàn phím (`knobKeyGesture`): F13 không kèm Shift là `press_rotate_right`, có Shift là
`press_rotate_left`. Phím F13 bị nuốt cả lúc nhấn lẫn lúc nhả, không tới ứng dụng khác. Bàn phím thường không có F13; nếu
bàn phím của bạn có F13 thật thì phím đó cũng bị coi là của núm. Muốn trả núm về như cũ, gán lại hai thao tác về
"tăng độ sáng" / "giảm độ sáng" trong phần mềm của hãng.

**Chỉnh độ sáng theo màn hình đang có con trỏ chuột** (giống macOS): gán thao tác tới `brightness_up` / `brightness_down`
(bảng Chọn nhanh, nhóm "Màn hình", hoặc mẫu nhanh "Độ sáng màn hình"). Hồ sơ mặc định đã gán giữ + xoay như vậy. Mỗi nấc 5%.

- Màn hình tích hợp của laptop: app nhận ra nó qua `QueryDisplayConfig` (kiểu kết nối nội bộ) và chỉnh qua thiết bị `\\.\LCD`
  của driver màn hình.
- Màn hình ngoài: lệnh DDC/CI qua `dxva2.dll`. Màn hình phải bật DDC/CI trong menu của chính nó. Màn hình đang có con trỏ
  không nhận lệnh thì app báo một lần ở khay.
- Mỗi lệnh DDC/CI mất khoảng 60 ms, nên việc này chạy ở luồng riêng và gộp các nấc xoay liên tiếp lại.
- Windows không hiện thanh độ sáng cho thay đổi kiểu này.
- Đã thử trên ThinkPad P1 Gen 5 với một màn hình ngoài (21/11/2026): cả hai màn hình tăng giảm 5% rồi về đúng mức cũ
  (`KNOB_TEST_DDC=1 go test ./windows -run Brightness -v`).
- Nếu núm chưa đổi sang F13 mà vẫn gán `brightness_up` / `brightness_down`, màn hình laptop đổi hai lần mỗi nấc (một lần
  của Windows, một lần của app) khi con trỏ ở màn hình laptop, và đổi theo Windows cả khi con trỏ ở màn hình ngoài.

### 9.6 Cửa sổ cài đặt

- **Cần Edge hoặc Chrome để có cửa sổ riêng.** Thiếu cả hai, trang mở thành tab trong trình duyệt mặc định; mọi chức năng vẫn chạy.
- **Kích thước cửa sổ:** `--window-size` chỉ có tác dụng lần mở đầu; về sau Edge nhớ vị trí và kích thước theo địa chỉ trang.
  Nếu cổng 47815 bận, app dùng cổng khác và Edge coi đó là một trang mới.
- **Ghi phím:** tổ hợp mà Windows hoặc trình duyệt giữ riêng (Alt + Tab, Win + D, Ctrl + W…) không ghi được bằng nút
  "Ghi phím…"; gõ tên phím vào ô Phím.
- **Danh sách "App đang chạy":** dựa vào `EnumWindows`; app UWP có thể hiện là `ApplicationFrameHost.exe`.
- **Icon app trong thanh bên:** hiện là ô màu kèm chữ cái đầu, chưa lấy icon thật của file exe.

### 9.7 App chạy quyền quản trị

`SendInput` từ app thường không tới được cửa sổ chạy quyền quản trị (UIPI), và có thể không đọc được tên exe của nó.
Nếu cần điều khiển các app đó, chạy `AnticaterKnob.exe` bằng quyền quản trị.

## 10. Việc nên làm tiếp

Theo thứ tự ưu tiên:

1. Chạy checklist mục 4 và sửa các lỗi lộ ra; đây là việc duy nhất thực sự chặn đường.
2. Đo độ lệch mốc thời gian giữa `WM_INPUT` và hook trên Windows, chỉnh `knobWindowMs` cho sát.
3. Thử cả ba kiểu kết nối (dây, dongle, Bluetooth), vì đường dẫn thiết bị và dạng gói có thể khác nhau.
4. Nhúng icon vào exe (file `.syso`), thay cho việc đọc icon từ file rời.
5. Hỏi lại khi đóng cửa sổ cài đặt mà chưa lưu; hiện icon thật của app trong thanh bên.
6. Chọn riêng từng màn hình khi chỉnh độ sáng thay vì tất cả. (Màn hình tích hợp của laptop đã chỉnh được qua `\\.\LCD`.)
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

## 12. Tài nguyên kèm theo

Mọi thứ dùng trong quá trình làm trên macOS đều có trên ổ này. Đường dẫn tính từ thư mục gốc repo.

| Vị trí | Nội dung | Trong git |
|---|---|---|
| `Sources/`, `build-gui.sh`, `README.md` | App macOS (Knob Console): mã nguồn, script build, tài liệu tiếng Anh | Có |
| `windows/` | App Windows này | Có (trừ exe, ico và file sinh ra khi chạy) |
| `linux/` | Cấu hình xremap, script cài và hướng dẫn cho Fedora KDE | Có |
| `assets/logo.svg`, `assets/make-icons.swift` | Logo vector và script sinh icon cho cả hai bản (chạy trên macOS) | Script có, logo không |
| `assets/AppIcon.icns`, `windows/AnticaterKnob.ico` | Icon đã sinh, màu cam `FF9124` | Không |
| `tools/mac-probes/` | Bốn công cụ chẩn đoán cho macOS đã dùng để tìm lỗi: liệt kê event tap, ghi phím và phím media, so mốc thời gian tín hiệu núm với phím hệ thống, liệt kê cửa sổ của app | Có |
| `local/app-mapping.mac.json` | Bản sao cấu hình Knob Console đang dùng trên Mac (hồ sơ Chrome, mặc định...) | Không |
| `local/vendor-AppIcon.icns` | Icon gốc màu xanh của app ANTICATER chính hãng | Không |
| `local/AppIcon-orange-recolored.icns` | Icon cam đổi màu trực tiếp từ icon gốc (bản đang gắn trong app Mac đang chạy) | Không |
| `../anticater-refs/` | Năm repo tham khảo: `anticater-dialer`, `vk01-anticater`, `antiknob`, `anticater-web`, `anticater-vk01-studio` | Repo riêng, nằm ngoài repo này |

Trong các repo tham khảo, `anticater-vk01-studio` là app Windows (C#) cho cùng chiếc núm, đáng đọc khi cần đối chiếu cách
Windows giao tín hiệu; `anticater-web` là trang cấu hình núm chạy trên Chrome, dùng được trên Windows.
