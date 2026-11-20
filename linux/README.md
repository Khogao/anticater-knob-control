# Anticater Knob trên Linux (Fedora KDE)

Trên Linux không cần viết app riêng: công cụ [xremap](https://github.com/xremap/xremap) đã làm đúng việc này.
Nó chiếm riêng một thiết bị nhập, đổi phím theo ứng dụng đang mở, và hỗ trợ KDE Plasma trên Wayland.
Thư mục này chỉ gồm cấu hình cho núm, script cài và hướng dẫn.

> **Trạng thái (2026-11-14): CHƯA CHẠY THỬ trên Linux.** Cấu hình và script được viết trên macOS dựa theo tài liệu
> của xremap (bản v0.15.18) và các dữ kiện đã đo về núm. Cú pháp script đã kiểm tra; phần còn lại phải thử trên máy thật.
> Những chỗ chưa chắc được ghi ở mục "Chưa kiểm chứng".

## Khác gì bản Mac và Windows

- **Không cần mẹo chặn phím.** Linux cho xremap chiếm riêng núm, nên phím gốc của núm không lọt ra ngoài.
- **Những gì không gán lại thì giữ nguyên.** Xoay vẫn là âm lượng, nhấn vẫn là tắt tiếng, nhấn giữ + xoay vẫn là
  độ sáng, do Plasma xử lý như khi không có xremap.
- **Không có cửa sổ cài đặt hay Chọn nhanh.** Cấu hình là một file YAML; lưu file là có hiệu lực ngay.
- **Không gửi được cuộn chuột.** xremap chỉ gửi phím, nên zoom dùng `Ctrl+=` / `Ctrl+-` thay cho Ctrl + cuộn.

## Cài đặt

### 1. Cài xremap

Tài liệu của xremap ghi có kho Copr cho Fedora:

```bash
sudo dnf copr enable blakegardner/xremap
```

```bash
dnf search xremap
```

Cài gói mà lệnh tìm kiếm liệt kê (tên gói và việc đã có bản cho Fedora 44 hay chưa thì mình chưa kiểm tra được):

```bash
sudo dnf install xremap
```

Nếu Copr chưa có bản cho Fedora 44, tải file nhị phân từ trang
[Releases](https://github.com/xremap/xremap/releases/latest) (bản đầy đủ, hỗ trợ mọi môi trường từ v0.15.13),
giải nén và chép `xremap` vào `~/.local/bin/`. Kiểm tra:

```bash
xremap --version
```

### 2. Cắm núm và chạy script cài

Núm phải đang kết nối (dây, dongle hoặc Bluetooth). Từ thư mục này:

```bash
./install.sh
```

Script làm các việc sau, và hỏi mật khẩu sudo cho hai việc đầu:

1. Thêm bạn vào nhóm `input`.
2. Tạo `/etc/udev/rules.d/99-input.rules` và `/etc/modules-load.d/uinput.conf`, để nhóm `input` dùng được `/dev/uinput`.
3. Tìm tên thiết bị của núm theo ID `05ac:022c` trong `/proc/bus/input/devices`.
4. Chép `config.yml` vào `~/.config/anticater-knob/` (giữ nguyên nếu đã có).
5. Tạo dịch vụ người dùng `anticater-knob.service`, chỉ chiếm núm, tự chạy cùng phiên Plasma.

Nếu ổ chứa thư mục này không giữ được quyền thực thi (ổ NTFS), chạy bằng `bash install.sh`.

Núm đời khác ID: `KNOB_VENDOR=514c KNOB_PRODUCT=8850 ./install.sh`.

### 3. Đăng xuất rồi đăng nhập lại

Việc thêm vào nhóm `input` chỉ có hiệu lực sau khi đăng nhập lại (hoặc khởi động lại). Sau đó dịch vụ tự chạy.

## Kiểm tra

```bash
systemctl --user status anticater-knob
```

```bash
journalctl --user -u anticater-knob -f
```

| Việc làm | Kỳ vọng |
|---|---|
| Xoay núm ở desktop | Âm lượng đổi như bình thường |
| Nhấn giữ + xoay ở desktop | Độ sáng màn hình đổi (nếu Plasma chỉnh được độ sáng màn hình của bạn) |
| Mở Chrome hoặc Firefox, nhấn giữ + xoay | Chuyển tab, độ sáng đứng yên |
| Bấm phím volume trên bàn phím thật | Không bị ảnh hưởng |

## Sửa cấu hình

File đang dùng: `~/.config/anticater-knob/config.yml`. Lưu là xremap tự nạp lại.

Phím núm gửi:

| Thao tác | Tên phím trong xremap |
|---|---|
| Xoay trái / phải | `VOLUMEDOWN` / `VOLUMEUP` |
| Nhấn | `MUTE` (một số đời: `PLAYPAUSE`) |
| Nhấn giữ + xoay trái / phải | `BRIGHTNESSDOWN` / `BRIGHTNESSUP` |

Một mục cấu hình:

```yaml
keymap:
  - name: Okular - xoay để zoom
    device:
      only: [ids:0x5ac:0x22c]      # chỉ áp dụng cho núm
    application:
      only: [/okular/]             # tên lớp cửa sổ; giữa hai dấu / là biểu thức chính quy
    remap:
      VOLUMEUP: C-Equal            # Ctrl+=
      VOLUMEDOWN: C-Minus          # Ctrl+-
```

- Phím bổ trợ: `C-` (Ctrl), `Shift-`, `Alt-`, `Super-`; ghép được, ví dụ `C-Shift-Tab`.
- Bỏ khối `application` để áp dụng ở mọi nơi.
- Để biết tên lớp của một cửa sổ: xem log bằng lệnh `journalctl` ở trên, đưa cửa sổ đó ra trước rồi xoay núm.
  xremap in `active window: caption: '...', class: '...'`. Nó chỉ in khi cấu hình có ít nhất một mục dùng `application`.
- `config.yml` có sẵn ba ví dụ đã chú thích: zoom trong trình xem tài liệu, tua video, chuyển bài.

## Chưa kiểm chứng

- **Gói trên Copr:** tên gói, và đã có bản cho Fedora 44 chưa.
- **Tên phím media:** `VOLUMEUP`, `BRIGHTNESSUP`... là tên chuẩn của nhân Linux mà xremap dùng, nhưng trang tài liệu tên phím
  của xremap không liệt kê riêng chúng. Nếu xremap báo lỗi tên phím, chạy `RUST_LOG=debug xremap ~/.config/anticater-knob/config.yml`
  rồi xoay núm để xem tên nó in ra.
- **Tên lớp cửa sổ của trình duyệt** trên Plasma (ví dụ `google-chrome`, `org.mozilla.firefox`); cấu hình dùng biểu thức
  chính quy rộng để khớp cả hai kiểu.
- **Tên thiết bị của núm trên Linux** và việc `--device` khớp theo phần đầu của tên. Nếu dịch vụ báo không tìm thấy thiết bị,
  chạy `xremap --list-devices` và sửa tên trong `~/.config/systemd/user/anticater-knob.service`.
- **Phát hiện ứng dụng trên Plasma 6:** tài liệu ghi xremap hỗ trợ KDE và phải chạy bằng người dùng thường (không sudo),
  đúng như dịch vụ ở đây.
- **Độ sáng màn hình ngoài:** tuỳ Plasma có điều khiển được màn hình của bạn hay không.

## Gỡ cài đặt

```bash
systemctl --user disable --now anticater-knob.service
```

```bash
rm ~/.config/systemd/user/anticater-knob.service
```

```bash
sudo rm /etc/udev/rules.d/99-input.rules /etc/modules-load.d/uinput.conf
```

```bash
sudo gpasswd -d "$USER" input
```

## Nếu muốn một app riêng như bản Windows

Phần lõi của `../windows/` (cấu hình JSON, chọn hành động theo app, bảng tên phím, Chọn nhanh, hai ngôn ngữ) viết bằng Go
và dùng lại được cho Linux. Phần phải viết mới: đọc núm bằng evdev, gửi phím bằng uinput, lấy cửa sổ đang mở từ KWin,
và giao diện. Chỉ nên làm khi cần cửa sổ cài đặt giống hai bản kia.
