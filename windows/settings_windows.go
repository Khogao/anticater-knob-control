package main

// Settings window: pick a profile (global or an app) on the left, edit its five
// gestures on the right, Save writes knob-config.json and applies it.

import (
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	idList = 10 + iota
	idAdd
	idDel
	idSave
	idApply
	idOpenFile

	idAddBlank = 999
	idAddFirst = 1000

	idQuickFirst = 200 // quick-pick combo of gesture row i has id idQuickFirst+i
)

type rowCtl struct{ typ, value, dy, dx, every, quick uintptr }

// quickPick is a ready-made action that can be dropped onto a gesture row.
type quickPick struct {
	label  string
	action Action
}

var quickPicks = []quickPick{
	{"Âm lượng +", Action{Type: "shortcut", Keys: []string{"volume_up"}}},
	{"Âm lượng −", Action{Type: "shortcut", Keys: []string{"volume_down"}}},
	{"Tắt tiếng", Action{Type: "shortcut", Keys: []string{"mute"}}},
	{"Play / Pause", Action{Type: "shortcut", Keys: []string{"play_pause"}}},
	{"Bài sau", Action{Type: "shortcut", Keys: []string{"next_track"}}},
	{"Bài trước", Action{Type: "shortcut", Keys: []string{"previous_track"}}},
	{"Zoom to (Ctrl + cuộn)", Action{Type: "scroll", Dy: 1, Keys: []string{"ctrl"}}},
	{"Zoom nhỏ (Ctrl + cuộn)", Action{Type: "scroll", Dy: -1, Keys: []string{"ctrl"}}},
	{"Cuộn lên", Action{Type: "scroll", Dy: 1}},
	{"Cuộn xuống", Action{Type: "scroll", Dy: -1}},
	{"Cuộn trái", Action{Type: "scroll", Dx: -1}},
	{"Cuộn phải", Action{Type: "scroll", Dx: 1}},
	{"Tab sau", Action{Type: "shortcut", Keys: []string{"ctrl", "tab"}}},
	{"Tab trước", Action{Type: "shortcut", Keys: []string{"ctrl", "shift", "tab"}}},
	{"Trang trước (Alt + ←)", Action{Type: "shortcut", Keys: []string{"alt", "left"}}},
	{"Trang sau (Alt + →)", Action{Type: "shortcut", Keys: []string{"alt", "right"}}},
	{"Tải lại (F5)", Action{Type: "shortcut", Keys: []string{"f5"}}},
	{"Hoàn tác (Ctrl + Z)", Action{Type: "shortcut", Keys: []string{"ctrl", "z"}}},
	{"Làm lại (Ctrl + Y)", Action{Type: "shortcut", Keys: []string{"ctrl", "y"}}},
	{"Sao chép (Ctrl + C)", Action{Type: "shortcut", Keys: []string{"ctrl", "c"}}},
	{"Dán (Ctrl + V)", Action{Type: "shortcut", Keys: []string{"ctrl", "v"}}},
	{"Lưu (Ctrl + S)", Action{Type: "shortcut", Keys: []string{"ctrl", "s"}}},
	{"Tìm (Ctrl + F)", Action{Type: "shortcut", Keys: []string{"ctrl", "f"}}},
	{"Mũi tên trái", Action{Type: "shortcut", Keys: []string{"left"}}},
	{"Mũi tên phải", Action{Type: "shortcut", Keys: []string{"right"}}},
	{"Enter", Action{Type: "shortcut", Keys: []string{"enter"}}},
	{"Esc", Action{Type: "shortcut", Keys: []string{"esc"}}},
	{"Space", Action{Type: "shortcut", Keys: []string{"space"}}},
	{"Chuyển cửa sổ (Alt + Tab)", Action{Type: "shortcut", Keys: []string{"alt", "tab"}}},
	{"Hiện desktop (Win + D)", Action{Type: "shortcut", Keys: []string{"win", "d"}}},
	{"Click trái", Action{Type: "mouse", Button: "left"}},
	{"Windows xử lý (như núm gốc)", Action{Type: "passthrough"}},
	{"Không làm gì", Action{Type: "noop"}},
}

type typeOption struct{ label, typ string }

var typeOptions = []typeOption{
	{"(theo mặc định)", ""},
	{"Windows xử lý (volume)", "passthrough"},
	{"Tổ hợp phím", "shortcut"},
	{"Cuộn chuột", "scroll"},
	{"Click chuột", "mouse"},
	{"Chạy lệnh", "shell"},
	{"Không làm gì", "noop"},
}

var gestureLabels = []string{"Xoay trái", "Xoay phải", "Nhấn", "Giữ + xoay trái", "Giữ + xoay phải"}

type template struct {
	label       string
	left, right Action
}

var templates = []template{
	{"Âm lượng (Windows)", Action{Type: "passthrough"}, Action{Type: "passthrough"}},
	{"Zoom (Ctrl + cuộn)", Action{Type: "scroll", Dy: -1, Keys: []string{"ctrl"}}, Action{Type: "scroll", Dy: 1, Keys: []string{"ctrl"}}},
	{"Zoom (Ctrl − / Ctrl =)", Action{Type: "shortcut", Keys: []string{"ctrl", "-"}}, Action{Type: "shortcut", Keys: []string{"ctrl", "="}}},
	{"Cuộn trang", Action{Type: "scroll", Dy: 1}, Action{Type: "scroll", Dy: -1}},
	{"Cuộn ngang", Action{Type: "scroll", Dx: -1}, Action{Type: "scroll", Dx: 1}},
	{"Chuyển tab", Action{Type: "shortcut", Keys: []string{"ctrl", "shift", "tab"}}, Action{Type: "shortcut", Keys: []string{"ctrl", "tab"}}},
	{"Tua video (← →)", Action{Type: "shortcut", Keys: []string{"left"}}, Action{Type: "shortcut", Keys: []string{"right"}}},
	{"Hoàn tác / làm lại", Action{Type: "shortcut", Keys: []string{"ctrl", "z"}}, Action{Type: "shortcut", Keys: []string{"ctrl", "y"}}},
	{"Cỡ brush ([ ])", Action{Type: "shortcut", Keys: []string{"["}}, Action{Type: "shortcut", Keys: []string{"]"}}},
	{"Bài trước / bài sau", Action{Type: "shortcut", Keys: []string{"previous_track"}}, Action{Type: "shortcut", Keys: []string{"next_track"}}},
}

var (
	hwndSettings    uintptr
	settingsClassOK bool
	uiFont          uintptr
	dpi             = 96

	editCfg    *Config
	profiles   []string // "" is global, anything else is an apps key
	curProfile = -1

	hList, hExe, hTitle, hStatus, hMonitor, hTemplate, hTarget uintptr
	rows                                                       [5]rowCtl
)

func sx(v int) uintptr { return uintptr(v * dpi / 96) }

func sendMsg(h uintptr, m uint32, w, l uintptr) uintptr {
	r, _, _ := pSendMessageW.Call(h, uintptr(m), w, l)
	return r
}

func setText(h uintptr, s string) {
	if h != 0 {
		pSetWindowTextW.Call(h, uintptr(unsafe.Pointer(u16(s))))
	}
}

func getText(h uintptr) string {
	n, _, _ := pGetWindowTextLengthW.Call(h)
	buf := make([]uint16, n+2)
	got, _, _ := pGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return strings.TrimSpace(syscall.UTF16ToString(buf[:got]))
}

func mk(class, text string, style, exStyle uint32, x, y, w, h, id int) uintptr {
	inst, _, _ := pGetModuleHandleW.Call(0)
	hwnd, _, _ := pCreateWindowExW.Call(uintptr(exStyle), uintptr(unsafe.Pointer(u16(class))), uintptr(unsafe.Pointer(u16(text))),
		uintptr(style|wsChild|wsVisible), sx(x), sx(y), sx(w), sx(h), hwndSettings, uintptr(id), inst, 0)
	sendMsg(hwnd, wmSetFont, uiFont, 1)
	return hwnd
}

func label(text string, x, y, w, h int) uintptr { return mk("STATIC", text, 0, 0, x, y, w, h, 0) }

func editBox(x, y, w, h int) uintptr {
	return mk("EDIT", "", wsTabStop|esAutoHScroll, wsExClientEdge, x, y, w, h, 0)
}

func combo(items []string, x, y, w int) uintptr { return comboID(items, x, y, w, 0) }

func comboID(items []string, x, y, w, id int) uintptr {
	h := mk("COMBOBOX", "", wsTabStop|wsVScroll|cbsDropDownList|cbsHasStrings, 0, x, y, w, 400, id)
	for _, it := range items {
		sendMsg(h, cbAddString, 0, uintptr(unsafe.Pointer(u16(it))))
	}
	sendMsg(h, cbSetCurSel, 0, 0)
	return h
}

func button(text string, x, y, w, h, id int) uintptr {
	return mk("BUTTON", text, wsTabStop, 0, x, y, w, h, id)
}

func profileLabel(key string) string {
	if key == "" {
		return "Mặc định (mọi app)"
	}
	return key
}

// initUI runs once at startup: DPI awareness, visual styles, font.
func initUI() {
	pSetProcessDPIAware.Call()
	if hdc, _, _ := pGetDC.Call(0); hdc != 0 {
		if v, _, _ := pGetDeviceCaps.Call(hdc, 88); v >= 96 && v <= 960 {
			dpi = int(v)
		}
		pReleaseDC.Call(0, hdc)
	}
	enableVisualStyles()
	uiFont, _, _ = pCreateFontW.Call(uintptr(-(9*dpi+36)/72)&0xFFFFFFFF, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(u16("Segoe UI"))))
}

type actCtx struct {
	Size         uint32
	Flags        uint32
	Source       *uint16
	Arch         uint16
	Lang         uint16
	AssemblyDir  *uint16
	ResourceName uintptr
	AppName      *uint16
	Module       uintptr
}

// enableVisualStyles activates the common-controls v6 manifest that ships
// inside shell32.dll, so controls get the modern look without embedding a
// manifest resource. Failure just leaves the classic look.
func enableVisualStyles() {
	buf := make([]uint16, 300)
	n, _, _ := pGetSystemDirectoryW.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 || int(n) >= len(buf) {
		return
	}
	sysDir := syscall.UTF16ToString(buf[:n])
	ctx := actCtx{Flags: 0x008 | 0x004, Source: u16(sysDir + `\shell32.dll`), AssemblyDir: u16(sysDir), ResourceName: 124}
	ctx.Size = uint32(unsafe.Sizeof(ctx))
	h, _, _ := pCreateActCtxW.Call(uintptr(unsafe.Pointer(&ctx)))
	if h == 0 || h == ^uintptr(0) {
		return
	}
	var cookie uintptr
	if ok, _, _ := pActivateActCtx.Call(h, uintptr(unsafe.Pointer(&cookie))); ok != 0 {
		pInitCommonControls.Call()
	}
}

func openSettings() {
	if hwndSettings != 0 {
		pShowWindow.Call(hwndSettings, 9) // SW_RESTORE
		pSetForegroundWindow.Call(hwndSettings)
		return
	}
	// Work on a copy so nothing changes until Save.
	data, _ := json.Marshal(cfg)
	editCfg = &Config{}
	json.Unmarshal(data, editCfg)
	if editCfg.Global == nil {
		editCfg.Global = map[string]Action{}
	}
	if editCfg.Apps == nil {
		editCfg.Apps = map[string]map[string]Action{}
	}
	profiles = []string{""}
	keys := make([]string, 0, len(editCfg.Apps))
	for k := range editCfg.Apps {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return strings.ToLower(keys[i]) < strings.ToLower(keys[j]) })
	profiles = append(profiles, keys...)
	curProfile = -1

	inst, _, _ := pGetModuleHandleW.Call(0)
	className := u16("AnticaterKnobSettings")
	if !settingsClassOK {
		cursor, _, _ := pLoadCursorW.Call(0, 32512)
		wc := wndClassEx{WndProc: syscall.NewCallback(settingsProc), Instance: inst, ClassName: className,
			Cursor: cursor, Icon: appIcon(false), IconSm: appIcon(true), Background: 16} // COLOR_BTNFACE + 1
		wc.Size = uint32(unsafe.Sizeof(wc))
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		settingsClassOK = true
	}
	hwndSettings, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(u16("Anticater Knob — Cài đặt"))), wsCaption|wsSysMenu|wsMinimizeBox,
		sx(120), sx(100), sx(1096), sx(440), 0, 0, inst, 0)
	if hwndSettings == 0 {
		return
	}

	hList = mk("LISTBOX", "", wsTabStop|wsVScroll|lbsNotify|lbsNoIntegralHeight, wsExClientEdge, 10, 10, 230, 300, idList)
	for _, p := range profiles {
		sendMsg(hList, lbAddString, 0, uintptr(unsafe.Pointer(u16(profileLabel(p)))))
	}
	button("Thêm app…", 10, 318, 112, 28, idAdd)
	button("Xoá", 128, 318, 112, 28, idDel)

	label("Tên exe:", 255, 14, 60, 20)
	hExe = editBox(318, 10, 170, 24)
	label("Tiêu đề chứa:", 498, 14, 90, 20)
	hTitle = editBox(590, 10, 170, 24)

	label("Thao tác", 255, 48, 120, 18)
	label("Hành động", 385, 48, 150, 18)
	label("Phím / nút chuột / lệnh", 545, 48, 200, 18)
	label("dy", 755, 48, 38, 18)
	label("dx", 800, 48, 38, 18)
	label("Mỗi", 845, 48, 38, 18)
	label("Chọn nhanh", 895, 48, 170, 18)

	quickLabels := []string{"Chọn nhanh…"}
	for _, q := range quickPicks {
		quickLabels = append(quickLabels, q.label)
	}
	typeLabels := make([]string, len(typeOptions))
	for i, o := range typeOptions {
		typeLabels[i] = o.label
	}
	for i := range rows {
		y := 70 + i*32
		label(gestureLabels[i], 255, y+4, 125, 20)
		rows[i] = rowCtl{
			typ:   combo(typeLabels, 385, y, 150),
			value: editBox(545, y, 200, 24),
			dy:    editBox(755, y, 38, 24),
			dx:    editBox(800, y, 38, 24),
			every: editBox(845, y, 38, 24),
			quick: comboID(quickLabels, 895, y, 180, idQuickFirst+i),
		}
	}

	label("Mẫu nhanh:", 255, 244, 72, 20)
	tl := make([]string, len(templates))
	for i, t := range templates {
		tl[i] = t.label
	}
	hTemplate = combo(tl, 330, 240, 190)
	label("cho", 528, 244, 25, 20)
	hTarget = combo([]string{"Xoay", "Giữ + xoay"}, 555, 240, 150)
	button("Áp dụng", 712, 239, 90, 26, idApply)

	label("Phím: ctrl shift alt win, chữ, số, f1…f24, left right up down, enter esc space tab, [ ] - = — nối bằng dấu +. "+
		"Cuộn: dy > 0 là lên, dx > 0 là sang phải; ô phím là phím giữ kèm (ctrl = zoom). "+
		"\"Mỗi\" = chỉ chạy mỗi N nấc; đặt 2 nếu một khấc núm của bạn chạy hành động hai lần.", 255, 275, 630, 50)
	hMonitor = label("Tín hiệu gần nhất: "+lastEventText, 255, 332, 630, 20)

	button("Lưu", 255, 358, 110, 30, idSave)
	button("Mở file JSON", 372, 358, 120, 30, idOpenFile)
	hStatus = label("", 505, 364, 380, 20)

	sendMsg(hList, lbSetCurSel, 0, 0)
	loadProfile(0)
	pShowWindow.Call(hwndSettings, 5) // SW_SHOW
	pSetForegroundWindow.Call(hwndSettings)
}

func profileActions(i int) map[string]Action {
	if profiles[i] == "" {
		return editCfg.Global
	}
	return editCfg.Apps[profiles[i]]
}

func setRow(i int, a Action, present bool) {
	sel := 0
	if present {
		sel = 1
		for j, o := range typeOptions {
			if o.typ != "" && o.typ == a.Type {
				sel = j
			}
		}
		if a.Type == "key" {
			sel = 2
		}
	}
	sendMsg(rows[i].typ, cbSetCurSel, uintptr(sel), 0)
	value := ""
	switch a.Type {
	case "shortcut", "key", "scroll":
		value = strings.Join(a.Keys, "+")
	case "mouse":
		value = a.Button
	case "shell":
		value = a.Command
	}
	setText(rows[i].value, value)
	setText(rows[i].dy, intText(a.Dy))
	setText(rows[i].dx, intText(a.Dx))
	setText(rows[i].every, intText(a.Every))
}

func intText(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

func loadProfile(i int) {
	curProfile = i
	exe, title, _ := strings.Cut(profiles[i], "|")
	isApp := profiles[i] != ""
	setText(hExe, exe)
	setText(hTitle, title)
	enable := uintptr(0)
	if isApp {
		enable = 1
	}
	pEnableWindow.Call(hExe, enable)
	pEnableWindow.Call(hTitle, enable)
	m := profileActions(i)
	for r, g := range gestureNames {
		a, ok := m[g]
		setRow(r, a, ok)
	}
}

func splitKeyList(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == '+' || r == ' ' })
}

// readRow turns one gesture row back into an action; ok is false for
// "(theo mặc định)", meaning the gesture is left out of the profile.
func readRow(i int) (a Action, ok bool) {
	sel := int(sendMsg(rows[i].typ, cbGetCurSel, 0, 0))
	if sel <= 0 || sel >= len(typeOptions) {
		return Action{}, false
	}
	a.Type = typeOptions[sel].typ
	value := getText(rows[i].value)
	switch a.Type {
	case "shortcut":
		a.Keys = splitKeyList(value)
	case "scroll":
		a.Keys = splitKeyList(value)
		a.Dy, _ = strconv.Atoi(getText(rows[i].dy))
		a.Dx, _ = strconv.Atoi(getText(rows[i].dx))
	case "mouse":
		a.Button = value
	case "shell":
		a.Command = value
	}
	if n, _ := strconv.Atoi(getText(rows[i].every)); n > 1 {
		a.Every = n
	}
	return a, true
}

func sameAction(a, b Action) bool {
	a.Description, b.Description = "", ""
	if a.Type == "key" {
		a.Type = "shortcut"
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// commitProfile copies the controls back into editCfg for the current profile.
func commitProfile() {
	if curProfile < 0 || hwndSettings == 0 {
		return
	}
	old := profileActions(curProfile)
	m := map[string]Action{}
	for r, g := range gestureNames {
		a, ok := readRow(r)
		if !ok {
			continue
		}
		if prev, had := old[g]; had && sameAction(prev, a) {
			a = prev // unchanged: keep its description
		}
		m[g] = a
	}
	oldKey := profiles[curProfile]
	if oldKey == "" {
		editCfg.Global = m
		return
	}
	newKey := oldKey
	if exe := getText(hExe); exe != "" {
		newKey = exe
		if title := getText(hTitle); title != "" {
			newKey += "|" + title
		}
	}
	if newKey != oldKey {
		if _, clash := editCfg.Apps[newKey]; clash {
			setText(hStatus, "Đã có mục "+newKey+" — giữ tên cũ.")
			newKey = oldKey
		} else {
			delete(editCfg.Apps, oldKey)
			profiles[curProfile] = newKey
			sendMsg(hList, lbDeleteString, uintptr(curProfile), 0)
			sendMsg(hList, lbInsertString, uintptr(curProfile), uintptr(unsafe.Pointer(u16(newKey))))
			sendMsg(hList, lbSetCurSel, uintptr(curProfile), 0)
		}
	}
	editCfg.Apps[newKey] = m
}

func selectProfile(i int) {
	sendMsg(hList, lbSetCurSel, uintptr(i), 0)
	loadProfile(i)
}

func addProfile(exe string) {
	commitProfile()
	for i, p := range profiles {
		if strings.EqualFold(p, exe) {
			selectProfile(i)
			return
		}
	}
	editCfg.Apps[exe] = map[string]Action{}
	profiles = append(profiles, exe)
	sendMsg(hList, lbAddString, 0, uintptr(unsafe.Pointer(u16(exe))))
	selectProfile(len(profiles) - 1)
}

func deleteProfile() {
	if curProfile <= 0 {
		setText(hStatus, "Không xoá được mục mặc định.")
		return
	}
	delete(editCfg.Apps, profiles[curProfile])
	sendMsg(hList, lbDeleteString, uintptr(curProfile), 0)
	profiles = append(profiles[:curProfile], profiles[curProfile+1:]...)
	curProfile = -1 // nothing to commit for the removed profile
	selectProfile(0)
}

var enumFound []string

func enumProc(hwnd, _ uintptr) uintptr {
	if vis, _, _ := pIsWindowVisible.Call(hwnd); vis == 0 {
		return 1
	}
	exe, title := windowApp(hwnd)
	if exe == "" || title == "" || strings.EqualFold(exe, selfExeName()) {
		return 1
	}
	for _, e := range enumFound {
		if strings.EqualFold(e, exe) {
			return 1
		}
	}
	enumFound = append(enumFound, exe)
	return 1
}

var enumCallback = syscall.NewCallback(enumProc)

func runningApps() []string {
	enumFound = nil
	pEnumWindows.Call(enumCallback, 0)
	sort.Slice(enumFound, func(i, j int) bool { return strings.ToLower(enumFound[i]) < strings.ToLower(enumFound[j]) })
	if len(enumFound) > 40 {
		enumFound = enumFound[:40]
	}
	return enumFound
}

func showAddMenu() {
	apps := runningApps()
	menu, _, _ := pCreatePopupMenu.Call()
	for i, a := range apps {
		pAppendMenuW.Call(menu, mfString, uintptr(idAddFirst+i), uintptr(unsafe.Pointer(u16(a))))
	}
	pAppendMenuW.Call(menu, mfSeparator, 0, 0)
	pAppendMenuW.Call(menu, mfString, idAddBlank, uintptr(unsafe.Pointer(u16("Mục trống (tự nhập tên exe)"))))
	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	cmd, _, _ := pTrackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd, uintptr(pt.X), uintptr(pt.Y), 0, hwndSettings, 0)
	pDestroyMenu.Call(menu)
	switch {
	case cmd == idAddBlank:
		name := "app.exe"
		for n := 2; ; n++ {
			if _, taken := editCfg.Apps[name]; !taken {
				break
			}
			name = "app" + strconv.Itoa(n) + ".exe"
		}
		addProfile(name)
		pSetFocus.Call(hExe)
	case cmd >= idAddFirst && int(cmd-idAddFirst) < len(apps):
		addProfile(apps[cmd-idAddFirst])
	}
}

func applyTemplate() {
	t := int(sendMsg(hTemplate, cbGetCurSel, 0, 0))
	if t < 0 || t >= len(templates) {
		return
	}
	left, right := 0, 1
	if sendMsg(hTarget, cbGetCurSel, 0, 0) == 1 {
		left, right = 3, 4
	}
	setRow(left, templates[t].left, true)
	setRow(right, templates[t].right, true)
	setText(hStatus, "Đã điền mẫu — bấm Lưu để áp dụng.")
}

func saveSettings() {
	commitProfile()
	if err := editCfg.validate(); err != nil {
		messageBox("Anticater Knob", "Chưa lưu được:\n"+err.Error())
		return
	}
	out, err := json.MarshalIndent(editCfg, "", "  ")
	if err == nil {
		err = os.WriteFile(configPath(), out, 0o644)
	}
	if err != nil {
		messageBox("Anticater Knob", "Không ghi được file cấu hình:\n"+err.Error())
		return
	}
	reloadConfig(false)
	setText(hStatus, "Đã lưu và áp dụng lúc "+time.Now().Format("15:04:05")+".")
}

func settingsProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmCommand:
		id, code := int(wParam&0xFFFF), int(wParam>>16&0xFFFF)
		switch {
		case id == idList && code == 1: // LBN_SELCHANGE
			sel := int(int32(sendMsg(hList, lbGetCurSel, 0, 0)))
			if sel >= 0 && sel < len(profiles) && sel != curProfile {
				commitProfile()
				selectProfile(sel)
			}
		case id >= idQuickFirst && id < idQuickFirst+len(rows) && code == 1: // CBN_SELCHANGE
			row := id - idQuickFirst
			pick := int(sendMsg(rows[row].quick, cbGetCurSel, 0, 0)) - 1
			if pick >= 0 && pick < len(quickPicks) {
				setRow(row, quickPicks[pick].action, true)
				setText(hStatus, "Đã gán \""+quickPicks[pick].label+"\" — bấm Lưu để áp dụng.")
			}
			sendMsg(rows[row].quick, cbSetCurSel, 0, 0)
		case code != 0:
		case id == idAdd:
			showAddMenu()
		case id == idDel:
			deleteProfile()
		case id == idApply:
			applyTemplate()
		case id == idSave:
			saveSettings()
		case id == idOpenFile:
			shellOpen(configPath())
		}
		return 0
	case wmClose:
		pDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		hwndSettings, curProfile = 0, -1
		hList, hExe, hTitle, hStatus, hMonitor, hTemplate, hTarget = 0, 0, 0, 0, 0, 0, 0
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}
