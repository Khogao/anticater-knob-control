# Anticater Knob cho Linux (KDE Plasma)

App cho núm Anticater VK-01 trên Linux: núm tự đổi chức năng theo ứng dụng đang mở, giống bản Windows.
Dùng chung định dạng cấu hình, năm thao tác và trang cài đặt với bản Windows (`../windows/`, `../ui/`).
Viết bằng Go, không cần cgo; chỉ dùng thêm thư viện D-Bus `github.com/godbus/dbus/v5`.

Cách làm cũ bằng xremap (chỉ file cấu hình, không có cửa sổ cài đặt) vẫn nằm trong [`xremap/`](xremap/).
Hai cách không chạy cùng lúc được, vì cả hai đều giữ riêng núm; `install.sh` ở đây tự tắt dịch vụ xremap cũ nếu có.

> **Trạng thái (2026-11-23):** đã build và chạy trên Fedora 44, KDE Plasma 6.8 Wayland. Phần nhận biết app đang mở,
> biểu tượng ở khay, trang cài đặt và thiết bị nhập ảo đã chạy thật trên máy. **Chưa thử với núm thật**, và phần đèn
> chưa gửi được lần nào (xem mục "Chưa kiểm chứng").

## Cài đặt

Cần Go (`sudo dnf install golang`). Từ thư mục này:

```bash
./install.sh
```

Script hỏi mật khẩu một lần (sudo) và làm năm việc:

1. Build app vào `~/.local/bin/anticater-knob`.
2. Ghi luật udev `/etc/udev/rules.d/60-anticater-knob.rules`: cho người đang đăng nhập đọc núm, giữ riêng núm, ghi lệnh đèn
   và tạo thiết bị nhập ảo. Luật chỉ áp dụng cho mã thiết bị của núm (`05AC:022C`, `514C:8850`, `514C:4155`).
3. Thêm biểu tượng và mục "Anticater Knob" vào menu ứng dụng.
4. Cho app tự chạy cùng Plasma (`~/.config/autostart/anticater-knob.desktop`).
5. Chạy app.

Không cần đăng xuất. Nếu núm đang cắm từ trước khi chạy script mà app vẫn báo "Chưa giữ được", rút ra cắm lại.

## Dùng

- Mở cài đặt: bấm biểu tượng núm ở khay hệ thống, hoặc mở "Anticater Knob" từ menu ứng dụng.
- Trang cài đặt giống bản Windows. Khác biệt: app được gọi theo **lớp cửa sổ** (window class) thay vì tên file exe,
  ví dụ `google-chrome`, `firefox`, `org.kde.okular`, `libreoffice-writer`. Mục "App đang chạy" liệt kê các app đã thấy.
- Thao tác để "Hệ điều hành xử lý" thì phím gốc của núm được gửi tiếp: Plasma đổi âm lượng, tắt tiếng, đổi độ sáng như thường.
- Cấu hình: `~/.config/anticater-knob/knob-config.json` (lưu từ trang cài đặt, hoặc sửa tay; app tự nạp lại).
- Log: `~/.config/anticater-knob/knob.log`. Báo cáo chẩn đoán: menu "Công cụ" trong trang cài đặt.
- Liệt kê thiết bị nhập và mã của chúng: `anticater-knob --list-devices`.

## Cách hoạt động

| Việc | Cách làm trên Linux | File |
|---|---|---|
| Đọc núm | Mở các nút `/dev/input/eventN` có mã của núm và giữ riêng (`EVIOCGRAB`), nên phím gốc của núm không tới app khác | `input_linux.go` |
| Gửi phím, cuộn, click | Một thiết bị nhập ảo tạo qua `/dev/uinput` ("Anticater Knob (virtual)") | `input_linux.go`, `actions_linux.go` |
| Biết app đang mở | Một script nhỏ chạy trong KWin báo lớp và tiêu đề cửa sổ đang hoạt động qua D-Bus (Wayland không cho app thường hỏi) | `kwin_linux.go` |
| Biểu tượng ở khay | StatusNotifierItem qua D-Bus | `tray_linux.go` |
| Trang cài đặt | Gói `ui` dùng chung, mở bằng Chrome/Edge chế độ app | `ui_linux.go` |
| Đèn | Lệnh `03 FD 00 06 R G B độ_sáng 01` của hãng: qua Bluetooth (BlueZ, đặc tính `AE41`) hoặc qua USB/dongle (`/dev/hidrawN`) | `led_linux.go`, `ledcolour_linux.go` |

Phần chọn màu (`ledcolour_linux.go`) và cấu hình (`config_linux.go`) chép từ bản Windows, sửa chỗ khác nhau.

## Chưa kiểm chứng

- **Núm thật:** mã phím núm gửi trên Linux. App coi `VOLUMEUP`/`VOLUMEDOWN` là xoay, `MUTE`/`PLAYPAUSE` là nhấn,
  `BRIGHTNESSUP`/`BRIGHTNESSDOWN` (hoặc `F13`/`Shift+F13`) là nhấn giữ + xoay. Chưa thử bằng núm thật.
- **Đèn qua USB/dongle:** chọn nút hidraw có báo cáo Output thuộc vùng của nhà sản xuất, ưu tiên mã báo cáo `03`, rồi ghi gói
  dài đúng kích thước báo cáo (65 byte nếu báo cáo dài 64 byte). Chưa ghi thử lần nào.
- **Đèn qua Bluetooth:** ghi vào đặc tính `AE41` qua BlueZ. Chưa thử.
- **Cuộn chuột** bằng thiết bị ảo: gửi cả sự kiện cuộn thường lẫn cuộn độ phân giải cao (120 mỗi nấc).

## Gỡ cài đặt

```bash
pkill -x anticater-knob
```

```bash
rm ~/.local/bin/anticater-knob ~/.config/autostart/anticater-knob.desktop ~/.local/share/applications/anticater-knob.desktop
```

```bash
sudo rm /etc/udev/rules.d/60-anticater-knob.rules
```
