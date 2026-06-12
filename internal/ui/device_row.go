package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/adb"
	"mirroid/internal/icons"
	"mirroid/internal/model"
)

// deviceRow is a custom widget for one row in the devices list. It holds
// typed refs to its dynamic children so the bind path doesn't need a
// 12-level index-chain of type assertions through fyne containers.
type deviceRow struct {
	widget.BaseWidget

	check      *widget.Check
	avatarBg   *canvas.Rectangle
	avatarIcon *canvas.Image
	nameTxt    *canvas.Text
	addrTxt    *canvas.Text
	statusSlot *fyne.Container

	serial  string                            // updated per bind
	onCheck func(serial string, checked bool) // captured at construction
}

func newDeviceRow(onCheck func(serial string, checked bool)) *deviceRow {
	r := &deviceRow{
		onCheck:    onCheck,
		check:      widget.NewCheck("", nil),
		avatarBg:   canvas.NewRectangle(pillGray),
		avatarIcon: canvas.NewImageFromResource(icons.NewTintedIcon(icons.SmartphoneIcon, color.White)),
		nameTxt:    canvas.NewText("", theme.Color(theme.ColorNameForeground)),
		addrTxt:    canvas.NewText("", theme.Color(theme.ColorNamePlaceHolder)),
		statusSlot: container.NewStack(),
	}
	r.avatarBg.CornerRadius = deviceRowAvatarSize / 2
	r.avatarIcon.SetMinSize(fyne.NewSize(deviceRowIconSize, deviceRowIconSize))
	r.avatarIcon.FillMode = canvas.ImageFillContain
	r.nameTxt.TextStyle = fyne.TextStyle{Bold: true}
	r.check.OnChanged = func(c bool) {
		if r.onCheck != nil {
			r.onCheck(r.serial, c)
		}
	}
	r.ExtendBaseWidget(r)
	return r
}

func (r *deviceRow) CreateRenderer() fyne.WidgetRenderer {
	avatarSquare := container.New(
		&fixedSizeLayout{width: deviceRowAvatarSize, height: deviceRowAvatarSize},
		container.NewStack(r.avatarBg, container.NewCenter(r.avatarIcon)),
	)
	avatar := container.NewCenter(avatarSquare)

	twoLine := container.New(&tightVLayout{spacing: 2}, r.nameTxt, r.addrTxt)

	leftGap := canvas.NewRectangle(nil)
	leftGap.SetMinSize(fyne.NewSize(theme.Padding(), 0))
	leftCluster := container.NewHBox(r.check, avatar, leftGap)

	rightGap := canvas.NewRectangle(nil)
	rightGap.SetMinSize(fyne.NewSize(theme.Padding(), 0))
	rightCluster := container.NewHBox(r.statusSlot, rightGap)

	root := container.NewPadded(container.NewBorder(nil, nil, leftCluster, rightCluster, twoLine))
	return widget.NewSimpleRenderer(root)
}

// bind updates the row's dynamic content from a device + its computed status.
func (r *deviceRow) bind(d adb.Device, status model.DeviceStatus, selected, checked bool) {
	r.serial = d.Serial

	pillBg := statusColor(status)
	r.avatarBg.FillColor = pillBg
	r.avatarBg.Refresh()

	if selected {
		r.nameTxt.Color = theme.Color(theme.ColorNamePrimary)
	} else {
		r.nameTxt.Color = theme.Color(theme.ColorNameForeground)
	}

	r.nameTxt.Text = deviceFriendlyName(d)
	r.nameTxt.Refresh()

	if brand := icons.BrandIcon(d.Manufacturer); brand != nil {
		r.avatarIcon.Resource = icons.NewTintedIcon(brand, color.White)
	} else {
		r.avatarIcon.Resource = icons.NewTintedIcon(icons.SmartphoneIcon, color.White)
	}
	r.avatarIcon.Refresh()

	r.addrTxt.Color = theme.Color(theme.ColorNamePlaceHolder)
	r.addrTxt.TextSize = theme.Size(theme.SizeNameCaptionText)
	r.addrTxt.Text = d.Serial + "  ·  " + connTypeLabel(d.Serial)
	r.addrTxt.Refresh()

	r.statusSlot.Objects = []fyne.CanvasObject{
		buildStatusBadge("● "+string(status), pillBg),
	}
	r.statusSlot.Refresh()

	// suppress the OnChanged callback while we set the check state imperatively
	saved := r.check.OnChanged
	r.check.OnChanged = nil
	r.check.SetChecked(checked)
	r.check.OnChanged = saved
}
