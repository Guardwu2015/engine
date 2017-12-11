// Copyright 2016 The G3N Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package gui

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/g3n/engine/gui/assets/icon"
	"github.com/g3n/engine/math32"
	"github.com/g3n/engine/window"
	"gopkg.in/yaml.v2"
)

// Builder builds GUI objects from a declarative description in YAML format
type Builder struct {
	desc    map[string]*descPanel // parsed descriptions
	imgpath string                // base path for image panels files
	objpath strStack              // stack of object names being built (used for error messages)
}

// descLayout contains all layout attributes
type descLayout struct {
	Type      string  // Type of the layout: HBox, VBox, Grid, Dock, others...
	Cols      int     // Number of columns for Grid layout
	Spacing   float32 // Spacing in pixels for HBox and VBox
	AlignH    string  // HBox group alignment type
	AlignV    string  // VBox group alignment type
	MinHeight bool    // HBox, VBox minimum height flag
	MinWidth  bool    // HBox, VBox minimum width flag
	ExpandH   bool    // Grid
	ExpandV   bool    // Grid
}

// descLayoutParam describes all layout parameters types
type descLayoutParams struct {
	Expand  *float32 // HBox, VBox expand factor
	ColSpan int      // Grid layout colspan
	AlignH  string   // horizontal alignment
	AlignV  string   // vertical alignment
	Edge    string   // Dock layout edge: top,right,bottom,left,center
}

// descPanel describes all panel types
type descPanel struct {
	Type         string            // Gui object type: Panel, Label, Edit, etc ...
	Name         string            // Optional name for identification
	Position     string            // Optional position as: x y | x,y
	Width        *float32          // Optional width (default = 0)
	Height       *float32          // Optional height (default = 0)
	AspectWidth  *float32          // Optional aspectwidth (default = nil)
	AspectHeight *float32          // Optional aspectwidth (default = nil)
	Margins      string            // Optional margins as 1 or 4 float values
	Borders      string            // Optional borders as 1 or 4 float values
	BorderColor  string            // Optional border color as name or 3 or 4 float values
	Paddings     string            // Optional paddings as 1 or 4 float values
	Color        string            // Optional color as 1 or 4 float values
	Enabled      *bool             // All:
	Visible      *bool             // All:
	Renderable   *bool             // All:
	Imagefile    string            // For Panel, Button
	Layout       *descLayout       // Optional pointer to layout
	LayoutParams *descLayoutParams // Optional layout parameters
	Text         string            // Label, Button
	Icons        string            // Label
	BgColor      string            // Label
	FontColor    string            // Label
	FontSize     *float32          // Label
	FontDPI      *float32          // Label
	LineSpacing  *float32          // Label
	PlaceHolder  string            // Edit
	MaxLength    *uint             // Edit
	Icon         string            // Button
	Group        string            // RadioButton
	Checked      bool              // CheckBox, RadioButton
	ImageLabel   *descPanel        // DropDown
	Items        []*descPanel      // Menu, MenuBar
	Shortcut     string            // Menu
	Value        *float32          // Slider
	ScaleFactor  *float32          // Slider
	Title        string            // Window
	Resizable    string            // Window resizable borders
	P0           *descPanel        // Splitter panel 0
	P1           *descPanel        // Splitter panel 1
	Split        *float32          // Splitter split value
	parent       *descPanel        // used internally
}

const (
	descTypePanel       = "panel"
	descTypeImagePanel  = "imagepanel"
	descTypeLabel       = "label"
	descTypeImageLabel  = "imagelabel"
	descTypeButton      = "button"
	descTypeCheckBox    = "checkbox"
	descTypeRadioButton = "radiobutton"
	descTypeEdit        = "edit"
	descTypeVList       = "vlist"
	descTypeHList       = "hlist"
	descTypeDropDown    = "dropdown"
	descTypeHSlider     = "hslider"
	descTypeVSlider     = "vslider"
	descTypeHSplitter   = "hsplitter"
	descTypeVSplitter   = "vsplitter"
	descTypeTree        = "tree"
	descTypeTreeNode    = "node"
	descTypeMenuBar     = "menubar"
	descTypeMenu        = "menu"
	descTypeWindow      = "window"
	descTypeHBoxLayout  = "hbox"
	descTypeVBoxLayout  = "vbox"
	descTypeGridLayout  = "grid"
	descTypeDockLayout  = "dock"
	fieldMargins        = "margins"
	fieldBorders        = "borders"
	fieldBorderColor    = "bordercolor"
	fieldPaddings       = "paddings"
	fieldColor          = "color"
	fieldBgColor        = "bgcolor"
)

const (
	aPOS         = 1 << iota                                  // attribute position
	aSIZE        = 1 << iota                                  // attribute size
	aNAME        = 1 << iota                                  // attribute name
	aMARGINS     = 1 << iota                                  // attribute margins widths
	aBORDERS     = 1 << iota                                  // attribute borders widths
	aBORDERCOLOR = 1 << iota                                  // attribute border color
	aPADDINGS    = 1 << iota                                  // attribute paddings widths
	aCOLOR       = 1 << iota                                  // attribute panel bgcolor
	aENABLED     = 1 << iota                                  // attribute enabled for events
	aRENDER      = 1 << iota                                  // attribute renderable
	aVISIBLE     = 1 << iota                                  // attribute visible
	asPANEL      = 0xFF                                       // attribute set for panels
	asWIDGET     = aPOS | aNAME | aENABLED | aVISIBLE         // attribute set for widgets
	asBUTTON     = aPOS | aSIZE | aNAME | aENABLED | aVISIBLE // attribute set for buttons
)

// maps align name with align parameter
var mapAlignName = map[string]Align{
	"none":   AlignNone,
	"left":   AlignLeft,
	"right":  AlignRight,
	"width":  AlignWidth,
	"top":    AlignTop,
	"bottom": AlignBottom,
	"height": AlignHeight,
	"center": AlignCenter,
}

// maps edge name (dock layout) with edge parameter
var mapEdgeName = map[string]int{
	"top":    DockTop,
	"right":  DockRight,
	"bottom": DockBottom,
	"left":   DockLeft,
	"center": DockCenter,
}

// maps resize border name (window) with parameter value
var mapResizable = map[string]Resizable{
	"top":    ResizeTop,
	"right":  ResizeRight,
	"bottom": ResizeBottom,
	"left":   ResizeLeft,
	"all":    ResizeAll,
}

// NewBuilder creates and returns a pointer to a new gui Builder object
func NewBuilder() *Builder {

	return new(Builder)
}

// ParseString parses a string with gui objects descriptions in YAML format
// It there was a previously parsed description, it is cleared.
func (b *Builder) ParseString(desc string) error {

	// Try assuming the description contains a single root panel
	var dp descPanel
	err := yaml.Unmarshal([]byte(desc), &dp)
	if err != nil {
		return err
	}
	if dp.Type != "" {
		b.desc = make(map[string]*descPanel)
		b.desc[""] = &dp
		b.setupDescTree(&dp)
		return nil
	}

	// Try assuming the description is a map of panels
	var dpm map[string]*descPanel
	err = yaml.Unmarshal([]byte(desc), &dpm)
	if err != nil {
		return err
	}
	b.desc = dpm
	for _, v := range dpm {
		b.setupDescTree(v)
	}
	return nil
}

// ParseFile parses a file with gui objects descriptions in YAML format
// It there was a previously parsed description, it is cleared.
func (b *Builder) ParseFile(filepath string) error {

	// Reads all file data
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}

	// Parses file data
	return b.ParseString(string(data))
}

// Names returns a sorted list of names of top level previously parsed objects.
// Only objects with defined types are returned.
// If there is only a single object with no name, its name is returned
// as an empty string
func (b *Builder) Names() []string {

	var objs []string
	for name, pd := range b.desc {
		if pd.Type != "" {
			objs = append(objs, name)
		}
	}
	sort.Strings(objs)
	return objs
}

// Build builds a gui object and all its children recursively.
// The specified name should be a top level name from a
// from a previously parsed description
// If the descriptions contains a single object with no name,
// It should be specified the empty string to build this object.
func (b *Builder) Build(name string) (IPanel, error) {

	pd, ok := b.desc[name]
	if !ok {
		return nil, fmt.Errorf("Object name:%s not found", name)
	}
	b.objpath.clear()
	b.objpath.push(pd.Name)
	return b.build(pd, nil)
}

// Sets the path for image panels relative image files
func (b *Builder) SetImagepath(path string) {

	b.imgpath = path
}

// build builds the gui object from the specified description.
// All its children are also built recursively
// Returns the built object or an error
func (b *Builder) build(pd *descPanel, iparent IPanel) (IPanel, error) {

	var err error
	var pan IPanel
	switch pd.Type {
	case descTypePanel:
		pan, err = b.buildPanel(pd)
	case descTypeImagePanel:
		pan, err = b.buildImagePanel(pd)
	case descTypeLabel:
		pan, err = b.buildLabel(pd)
	case descTypeImageLabel:
		pan, err = b.buildImageLabel(pd)
	case descTypeButton:
		pan, err = b.buildButton(pd)
	case descTypeCheckBox:
		pan, err = b.buildCheckBox(pd)
	case descTypeRadioButton:
		pan, err = b.buildRadioButton(pd)
	case descTypeEdit:
		pan, err = b.buildEdit(pd)
	case descTypeVList:
		pan, err = b.buildVList(pd)
	case descTypeHList:
		pan, err = b.buildHList(pd)
	case descTypeDropDown:
		pan, err = b.buildDropDown(pd)
	case descTypeHSlider:
		pan, err = b.buildSlider(pd, true)
	case descTypeVSlider:
		pan, err = b.buildSlider(pd, false)
	case descTypeHSplitter:
		pan, err = b.buildSplitter(pd, true)
	case descTypeVSplitter:
		pan, err = b.buildSplitter(pd, false)
	case descTypeTree:
		pan, err = b.buildTree(pd)
	case descTypeMenuBar:
		pan, err = b.buildMenu(pd, false, true)
	case descTypeMenu:
		pan, err = b.buildMenu(pd, false, false)
	case descTypeWindow:
		pan, err = b.buildWindow(pd)
	default:
		err = fmt.Errorf("Invalid panel type:%s", pd.Type)
	}
	if err != nil {
		return nil, err
	}
	if iparent != nil {
		iparent.GetPanel().Add(pan)
	}
	return pan, nil
}

// buildPanel builds a gui object of type: "Panel"
func (b *Builder) buildPanel(dp *descPanel) (IPanel, error) {

	// Builds panel and set attributes
	width, height := b.size(dp)
	pan := NewPanel(width, height)
	err := b.setAttribs(dp, pan, asPANEL)
	if err != nil {
		return nil, err
	}

	// Builds panel children recursively
	for i := 0; i < len(dp.Items); i++ {
		item := dp.Items[i]
		b.objpath.push(item.Name)
		child, err := b.build(item, pan)
		b.objpath.pop()
		if err != nil {
			return nil, err
		}
		pan.Add(child)
	}
	return pan, nil
}

// buildImagePanel builds a gui object of type: "ImagePanel"
func (b *Builder) buildImagePanel(pd *descPanel) (IPanel, error) {

	// Imagefile must be supplied
	if pd.Imagefile == "" {
		return nil, b.err("Imagefile", "Imagefile must be supplied")
	}

	// If path is not absolute join with user supplied image base path
	path := pd.Imagefile
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.imgpath, path)
	}

	// Builds panel and set common attributes
	panel, err := NewImage(path)
	if err != nil {
		return nil, err
	}
	err = b.setAttribs(pd, panel, asPANEL)
	if err != nil {
		return nil, err
	}

	// AspectWidth and AspectHeight attributes
	if pd.AspectWidth != nil {
		panel.SetContentAspectWidth(*pd.AspectWidth)
	}
	if pd.AspectHeight != nil {
		panel.SetContentAspectHeight(*pd.AspectHeight)
	}

	// Builds panel children recursively
	for i := 0; i < len(pd.Items); i++ {
		item := pd.Items[i]
		b.objpath.push(item.Name)
		child, err := b.build(item, panel)
		b.objpath.pop()
		if err != nil {
			return nil, err
		}
		panel.Add(child)
	}
	return panel, nil
}

// buildLabel builds a gui object of type: "Label"
func (b *Builder) buildLabel(pd *descPanel) (IPanel, error) {

	// Builds label with icon or text font
	var label *Label
	icons, err := b.parseIconNames("icons", pd.Icons)
	if err != nil {
		return nil, err
	}
	if icons != "" {
		label = NewLabel(icons, true)
	} else {
		label = NewLabel(pd.Text)
	}
	// Sets common attributes
	err = b.setAttribs(pd, label, asPANEL)
	if err != nil {
		return nil, err
	}

	// Set optional background color
	c, err := b.parseColor(fieldBgColor, pd.BgColor)
	if err != nil {
		return nil, err
	}
	if c != nil {
		label.SetBgColor4(c)
	}

	// Set optional font color
	c, err = b.parseColor("fontcolor", pd.FontColor)
	if err != nil {
		return nil, err
	}
	if c != nil {
		label.SetColor4(c)
	}

	// Sets optional font size
	if pd.FontSize != nil {
		label.SetFontSize(float64(*pd.FontSize))
	}

	// Sets optional font dpi
	if pd.FontDPI != nil {
		label.SetFontDPI(float64(*pd.FontDPI))
	}

	// Sets optional line spacing
	if pd.LineSpacing != nil {
		label.SetLineSpacing(float64(*pd.LineSpacing))
	}

	return label, nil
}

// buildImageLabel builds a gui object of type: ImageLabel
func (b *Builder) buildImageLabel(pd *descPanel) (IPanel, error) {

	// Builds image label and set common attributes
	imglabel := NewImageLabel(pd.Text)
	err := b.setAttribs(pd, imglabel, asPANEL)
	if err != nil {
		return nil, err
	}

	// Sets optional icon(s)
	icons, err := b.parseIconNames("icons", pd.Icons)
	if err != nil {
		return nil, err
	}
	if icons != "" {
		imglabel.SetIcon(icons)
	}

	// Sets optional image from file
	// If path is not absolute join with user supplied image base path
	if pd.Imagefile != "" {
		path := pd.Imagefile
		if !filepath.IsAbs(path) {
			path = filepath.Join(b.imgpath, path)
		}
		err := imglabel.SetImageFromFile(path)
		if err != nil {
			return nil, err
		}
	}

	return imglabel, nil
}

// buildButton builds a gui object of type: Button
func (b *Builder) buildButton(pd *descPanel) (IPanel, error) {

	// Builds button and set commont attributes
	button := NewButton(pd.Text)
	err := b.setAttribs(pd, button, asBUTTON)
	if err != nil {
		return nil, err
	}

	// Sets optional icon
	if pd.Icon != "" {
		cp, err := b.parseIconName("icon", pd.Icon)
		if err != nil {
			return nil, err
		}
		button.SetIcon(cp)
	}

	// Sets optional image from file
	// If path is not absolute join with user supplied image base path
	if pd.Imagefile != "" {
		path := pd.Imagefile
		if !filepath.IsAbs(path) {
			path = filepath.Join(b.imgpath, path)
		}
		err := button.SetImage(path)
		if err != nil {
			return nil, err
		}
	}

	return button, nil
}

// buildCheckBox builds a gui object of type: CheckBox
func (b *Builder) buildCheckBox(pd *descPanel) (IPanel, error) {

	// Builds check box and set commont attributes
	cb := NewCheckBox(pd.Text)
	err := b.setAttribs(pd, cb, asWIDGET)
	if err != nil {
		return nil, err
	}
	cb.SetValue(pd.Checked)
	return cb, nil
}

// buildRadioButton builds a gui object of type: RadioButton
func (b *Builder) buildRadioButton(pd *descPanel) (IPanel, error) {

	// Builds check box and set commont attributes
	rb := NewRadioButton(pd.Text)
	err := b.setAttribs(pd, rb, asWIDGET)
	if err != nil {
		return nil, err
	}

	// Sets optional radio button group
	if pd.Group != "" {
		rb.SetGroup(pd.Group)
	}
	rb.SetValue(pd.Checked)
	return rb, nil
}

// buildEdit builds a gui object of type: "Edit"
func (b *Builder) buildEdit(dp *descPanel) (IPanel, error) {

	// Builds button and set attributes
	width, _ := b.size(dp)
	edit := NewEdit(int(width), dp.PlaceHolder)
	err := b.setAttribs(dp, edit, asWIDGET)
	if err != nil {
		return nil, err
	}
	edit.SetText(dp.Text)
	return edit, nil
}

// buildVList builds a gui object of type: VList
func (b *Builder) buildVList(dp *descPanel) (IPanel, error) {

	// Builds list and set commont attributes
	width, height := b.size(dp)
	list := NewVList(width, height)
	err := b.setAttribs(dp, list, asWIDGET)
	if err != nil {
		return nil, err
	}

	// Builds list children
	for i := 0; i < len(dp.Items); i++ {
		item := dp.Items[i]
		b.objpath.push(item.Name)
		child, err := b.build(item, list)
		b.objpath.pop()
		if err != nil {
			return nil, err
		}
		list.Add(child)
	}
	return list, nil
}

// buildHList builds a gui object of type: VList
func (b *Builder) buildHList(dp *descPanel) (IPanel, error) {

	// Builds list and set commont attributes
	width, height := b.size(dp)
	list := NewHList(width, height)
	err := b.setAttribs(dp, list, asWIDGET)
	if err != nil {
		return nil, err
	}

	// Builds list children
	for i := 0; i < len(dp.Items); i++ {
		item := dp.Items[i]
		b.objpath.push(item.Name)
		child, err := b.build(item, list)
		b.objpath.pop()
		if err != nil {
			return nil, err
		}
		list.Add(child)
	}
	return list, nil
}

// buildDropDown builds a gui object of type: DropDown
func (b *Builder) buildDropDown(pd *descPanel) (IPanel, error) {

	// If image label attribute defined use it, otherwise
	// uses default value.
	var imglabel *ImageLabel
	if pd.ImageLabel != nil {
		pd.ImageLabel.Type = descTypeImageLabel
		ipan, err := b.build(pd.ImageLabel, nil)
		if err != nil {
			return nil, err
		}
		imglabel = ipan.(*ImageLabel)
	} else {
		imglabel = NewImageLabel("")
	}

	// Builds drop down and set common attributes
	width, _ := b.size(pd)
	dd := NewDropDown(width, imglabel)
	err := b.setAttribs(pd, dd, asWIDGET)
	if err != nil {
		return nil, err
	}

	// Builds drop down children
	for i := 0; i < len(pd.Items); i++ {
		item := pd.Items[i]
		item.Type = descTypeImageLabel
		b.objpath.push(item.Name)
		child, err := b.build(item, dd)
		b.objpath.pop()
		if err != nil {
			return nil, err
		}
		dd.Add(child.(*ImageLabel))
	}
	return dd, nil
}

// buildSlider builds a gui object of type: HSlider or VSlider
func (b *Builder) buildSlider(pd *descPanel, horiz bool) (IPanel, error) {

	// Builds slider and sets its position
	width, height := b.size(pd)
	var slider *Slider
	if horiz {
		slider = NewHSlider(width, height)
	} else {
		slider = NewVSlider(width, height)
	}
	err := b.setAttribs(pd, slider, asWIDGET)
	if err != nil {
		return nil, err
	}

	// Sets optional text
	if pd.Text != "" {
		slider.SetText(pd.Text)
	}
	// Sets optional scale factor
	if pd.ScaleFactor != nil {
		slider.SetScaleFactor(*pd.ScaleFactor)
	}
	// Sets optional value
	if pd.Value != nil {
		slider.SetValue(*pd.Value)
	}
	return slider, nil
}

// buildSplitter builds a gui object of type: HSplitterr or VSplitter
func (b *Builder) buildSplitter(pd *descPanel, horiz bool) (IPanel, error) {

	// Builds splitter and sets its common attributes
	width, height := b.size(pd)
	var splitter *Splitter
	if horiz {
		splitter = NewHSplitter(width, height)
	} else {
		splitter = NewVSplitter(width, height)
	}
	err := b.setAttribs(pd, splitter, asWIDGET)
	if err != nil {
		return nil, err
	}

	// Optional split value
	if pd.Split != nil {
		splitter.SetSplit(*pd.Split)
	}

	// Splitter panel 0 attributes and items
	if pd.P0 != nil {
		err := b.setAttribs(pd.P0, &splitter.P0, asPANEL)
		if err != nil {
			return nil, err
		}
		err = b.addPanelItems(pd.P0, &splitter.P0)
		if err != nil {
			return nil, err
		}
	}

	// Splitter panel 1 attributes and items
	if pd.P1 != nil {
		err := b.setAttribs(pd.P1, &splitter.P1, asPANEL)
		if err != nil {
			return nil, err
		}
		err = b.addPanelItems(pd.P1, &splitter.P1)
		if err != nil {
			return nil, err
		}
	}

	return splitter, nil
}

// buildTree builds a gui object of type: Tree
func (b *Builder) buildTree(dp *descPanel) (IPanel, error) {

	// Builds tree and sets its common attributes
	width, height := b.size(dp)
	tree := NewTree(width, height)
	err := b.setAttribs(dp, tree, asWIDGET)
	if err != nil {
		return nil, err
	}

	// Internal function to build tree nodes recursively
	var buildItems func(dp *descPanel, pnode *TreeNode) error
	buildItems = func(dp *descPanel, pnode *TreeNode) error {
		for i := 0; i < len(dp.Items); i++ {
			item := dp.Items[i]
			// Item is a tree node
			if item.Type == "" || item.Type == descTypeTreeNode {
				var node *TreeNode
				if pnode == nil {
					node = tree.AddNode(item.Text)
				} else {
					node = pnode.AddNode(item.Text)
				}
				err := buildItems(item, node)
				if err != nil {
					return err
				}
				continue
			}
			// Other controls
			ipan, err := b.build(item, nil)
			if err != nil {
				return err
			}
			if pnode == nil {
				tree.Add(ipan)
			} else {
				pnode.Add(ipan)
			}
		}
		return nil
	}

	// Build nodes
	err = buildItems(dp, nil)
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// buildMenu builds a gui object of type: Menu or MenuBar from the
// specified panel descriptor.
func (b *Builder) buildMenu(pd *descPanel, child, bar bool) (IPanel, error) {

	// Builds menu bar or menu
	var menu *Menu
	if bar {
		menu = NewMenuBar()
	} else {
		menu = NewMenu()
	}
	// Only sets attribs for top level menus
	if !child {
		err := b.setAttribs(pd, menu, asWIDGET)
		if err != nil {
			return nil, err
		}
	}

	// Builds and adds menu items
	for i := 0; i < len(pd.Items); i++ {
		item := pd.Items[i]
		// Item is another menu
		if item.Type == descTypeMenu {
			subm, err := b.buildMenu(item, true, false)
			if err != nil {
				return nil, err
			}
			menu.AddMenu(item.Text, subm.(*Menu))
			continue
		}
		// Item is a separator
		if item.Type == "Separator" {
			menu.AddSeparator()
			continue
		}
		// Item must be a menu option
		mi := menu.AddOption(item.Text)
		// Set item optional icon(s)
		icons, err := b.parseIconNames("icon", item.Icon)
		if err != nil {
			return nil, err
		}
		if icons != "" {
			mi.SetIcon(string(icons))
		}
		// Sets optional menu item shortcut
		err = b.setMenuShortcut(mi, "shortcut", item.Shortcut)
		if err != nil {
			return nil, err
		}
	}
	return menu, nil
}

// buildWindow builds a gui object of type: Window from the
// specified panel descriptor.
func (b *Builder) buildWindow(dp *descPanel) (IPanel, error) {

	// Builds window and sets its common attributes
	width, height := b.size(dp)
	win := NewWindow(width, height)
	err := b.setAttribs(dp, win, asWIDGET)
	if err != nil {
		return nil, err
	}

	// Title attribute
	win.SetTitle(dp.Title)

	// Parse resizable borders
	if dp.Resizable != "" {
		parts := strings.Fields(dp.Resizable)
		var res Resizable
		for _, name := range parts {
			v, ok := mapResizable[name]
			if !ok {
				return nil, b.err("resizable", "Invalid resizable name:"+name)
			}
			res |= v
		}
		win.SetResizable(res)
	}

	// Builds window client panel children recursively
	for i := 0; i < len(dp.Items); i++ {
		item := dp.Items[i]
		b.objpath.push(item.Name)
		child, err := b.build(item, win)
		b.objpath.pop()
		if err != nil {
			return nil, err
		}
		win.Add(child)
	}
	return win, nil
}

// addPanelItems adds the items in the panel descriptor to the specified panel
func (b *Builder) addPanelItems(dp *descPanel, ipan IPanel) error {

	pan := ipan.GetPanel()
	for i := 0; i < len(dp.Items); i++ {
		item := dp.Items[i]
		b.objpath.push(item.Name)
		child, err := b.build(item, pan)
		b.objpath.pop()
		if err != nil {
			return err
		}
		pan.Add(child)
	}
	return nil
}

// setAttribs sets common attributes from the description to the specified panel
// The attributes which are set can be specified by the specified bitmask.
func (b *Builder) setAttribs(pd *descPanel, ipan IPanel, attr uint) error {

	panel := ipan.GetPanel()
	// Set optional position
	if attr&aPOS != 0 && pd.Position != "" {
		va, err := b.parseFloats("position", pd.Position, 2, 2)
		if va == nil || err != nil {
			return err
		}
		panel.SetPosition(va[0], va[1])
	}

	// Set optional size
	if attr&aSIZE != 0 {
		if pd.Width != nil {
			panel.SetWidth(*pd.Width)
		}
		if pd.Height != nil {
			panel.SetHeight(*pd.Height)
		}
	}

	// Set optional margin sizes
	if attr&aMARGINS != 0 {
		bs, err := b.parseBorderSizes(fieldMargins, pd.Margins)
		if err != nil {
			return err
		}
		if bs != nil {
			panel.SetMarginsFrom(bs)
		}
	}

	// Set optional border sizes
	if attr&aBORDERS != 0 {
		bs, err := b.parseBorderSizes(fieldBorders, pd.Borders)
		if err != nil {
			return err
		}
		if bs != nil {
			panel.SetBordersFrom(bs)
		}
	}

	// Set optional border color
	if attr&aBORDERCOLOR != 0 {
		c, err := b.parseColor(fieldBorderColor, pd.BorderColor)
		if err != nil {
			return err
		}
		if c != nil {
			panel.SetBordersColor4(c)
		}
	}

	// Set optional paddings sizes
	if attr&aPADDINGS != 0 {
		bs, err := b.parseBorderSizes(fieldPaddings, pd.Paddings)
		if err != nil {
			return err
		}
		if bs != nil {
			panel.SetPaddingsFrom(bs)
		}
	}

	// Set optional color
	if attr&aCOLOR != 0 {
		c, err := b.parseColor(fieldColor, pd.Color)
		if err != nil {
			return err
		}
		if c != nil {
			panel.SetColor4(c)
		}
	}

	if attr&aNAME != 0 && pd.Name != "" {
		panel.SetName(pd.Name)
	}
	if attr&aVISIBLE != 0 && pd.Visible != nil {
		panel.SetVisible(*pd.Visible)
	}
	if attr&aENABLED != 0 && pd.Enabled != nil {
		panel.SetEnabled(*pd.Enabled)
	}
	if attr&aRENDER != 0 && pd.Renderable != nil {
		panel.SetRenderable(*pd.Renderable)
	}

	err := b.setLayoutParams(pd, ipan)
	if err != nil {
		return err
	}
	return b.setLayout(pd, ipan)
}

// setLayoutParams sets the optional layout params attribute for specified the panel
func (b *Builder) setLayoutParams(dp *descPanel, ipan IPanel) error {

	// If layout params not declared, nothing to do
	if dp.LayoutParams == nil {
		return nil
	}

	// Get the parent layout
	if dp.parent == nil {
		return b.err("layoutparams", "No parent defined")
	}
	playout := dp.parent.Layout
	if playout == nil {
		return b.err("layoutparams", "Parent does not have layout")
	}
	panel := ipan.GetPanel()
	dlp := dp.LayoutParams

	// HBoxLayout parameters
	if playout.Type == descTypeHBoxLayout {
		// Creates layout parameter
		params := HBoxLayoutParams{Expand: 0, AlignV: AlignTop}
		// Sets optional expand parameter
		if dlp.Expand != nil {
			params.Expand = *dlp.Expand
		}
		// Sets optional align parameter
		if dlp.AlignV != "" {
			align, ok := mapAlignName[dlp.AlignV]
			if !ok {
				return b.err("align", "Invalid align name:"+dlp.AlignV)
			}
			params.AlignV = align
		}
		panel.SetLayoutParams(&params)
		return nil
	}

	// VBoxLayout parameters
	if playout.Type == descTypeVBoxLayout {
		// Creates layout parameter
		params := VBoxLayoutParams{Expand: 0, AlignH: AlignLeft}
		// Sets optional expand parameter
		if dlp.Expand != nil {
			params.Expand = *dlp.Expand
		}
		// Sets optional align parameter
		if dlp.AlignH != "" {
			align, ok := mapAlignName[dlp.AlignH]
			if !ok {
				return b.err("align", "Invalid align name:"+dlp.AlignH)
			}
			params.AlignH = align
		}
		panel.SetLayoutParams(&params)
		return nil
	}

	// GridLayout parameters
	if playout.Type == descTypeGridLayout {
		// Creates layout parameter
		params := GridLayoutParams{
			ColSpan: 0,
			AlignH:  AlignNone,
			AlignV:  AlignNone,
		}
		params.ColSpan = dlp.ColSpan
		// Sets optional alignh parameter
		if dlp.AlignH != "" {
			align, ok := mapAlignName[dlp.AlignH]
			if !ok {
				return b.err("alignh", "Invalid align name:"+dlp.AlignH)
			}
			params.AlignH = align
		}
		// Sets optional alignv parameter
		if dlp.AlignV != "" {
			align, ok := mapAlignName[dlp.AlignV]
			if !ok {
				return b.err("alignv", "Invalid align name:"+dlp.AlignV)
			}
			params.AlignV = align
		}
		panel.SetLayoutParams(&params)
		return nil
	}

	// DockLayout parameters
	if playout.Type == descTypeDockLayout {
		if dlp.Edge != "" {
			edge, ok := mapEdgeName[dlp.Edge]
			if !ok {
				return b.err("edge", "Invalid edge name:"+dlp.Edge)
			}
			params := DockLayoutParams{Edge: edge}
			panel.SetLayoutParams(&params)
			return nil
		}
	}

	return b.err("layoutparams", "Invalid parent layout:"+playout.Type)
}

// setLayout sets the optional panel layout and layout parameters
func (b *Builder) setLayout(dp *descPanel, ipan IPanel) error {

	// If layout types not declared, nothing to do
	if dp.Layout == nil {
		return nil
	}
	dl := dp.Layout

	// HBox layout
	if dl.Type == descTypeHBoxLayout {
		hbl := NewHBoxLayout()
		hbl.SetSpacing(dl.Spacing)
		if dl.AlignH != "" {
			align, ok := mapAlignName[dl.AlignH]
			if !ok {
				return b.err("align", "Invalid align name:"+dl.AlignV)
			}
			hbl.SetAlignH(align)
		}
		hbl.SetMinHeight(dl.MinHeight)
		hbl.SetMinWidth(dl.MinWidth)
		ipan.SetLayout(hbl)
		return nil
	}

	// VBox layout
	if dl.Type == descTypeVBoxLayout {
		vbl := NewVBoxLayout()
		vbl.SetSpacing(dl.Spacing)
		if dl.AlignV != "" {
			align, ok := mapAlignName[dl.AlignV]
			if !ok {
				return b.err("align", "Invalid align name:"+dl.AlignV)
			}
			vbl.SetAlignV(align)
		}
		vbl.SetMinHeight(dl.MinHeight)
		vbl.SetMinWidth(dl.MinWidth)
		ipan.SetLayout(vbl)
		return nil
	}

	// Grid layout
	if dl.Type == descTypeGridLayout {
		// Number of columns
		if dl.Cols == 0 {
			return b.err("cols", "Invalid number of columns:"+dl.AlignH)
		}
		grl := NewGridLayout(dl.Cols)
		// Global horizontal alignment
		if dl.AlignH != "" {
			alignh, ok := mapAlignName[dl.AlignH]
			if !ok {
				return b.err("alignh", "Invalid horizontal align:"+dl.AlignH)
			}
			grl.SetAlignH(alignh)
		}
		// Global vertical alignment
		if dl.AlignV != "" {
			alignv, ok := mapAlignName[dl.AlignV]
			if !ok {
				return b.err("alignv", "Invalid vertical align:"+dl.AlignH)
			}
			grl.SetAlignV(alignv)
		}
		// Expansion flags
		grl.SetExpandH(dl.ExpandH)
		grl.SetExpandV(dl.ExpandV)
		ipan.SetLayout(grl)
		return nil
	}

	// Dock layout
	if dl.Type == descTypeDockLayout {
		dockl := NewDockLayout()
		ipan.SetLayout(dockl)
		return nil
	}

	return b.err("layout", "Invalid layout type:"+dl.Type)
}

func (b *Builder) setMenuShortcut(mi *MenuItem, fname, field string) error {

	field = strings.Trim(field, " ")
	if field == "" {
		return nil
	}
	parts := strings.Split(field, "+")
	var mods window.ModifierKey
	for i := 0; i < len(parts)-1; i++ {
		switch parts[i] {
		case "Shift":
			mods |= window.ModShift
		case "Ctrl":
			mods |= window.ModControl
		case "Alt":
			mods |= window.ModAlt
		default:
			return b.err(fname, "Invalid shortcut:"+field)
		}
	}
	// The last part must be a key
	key := parts[len(parts)-1]
	for kcode, kname := range mapKeyText {
		if kname == key {
			mi.SetShortcut(mods, kcode)
			return nil
		}
	}
	return b.err(fname, "Invalid shortcut:"+field)
}

// parseBorderSizes parses a string field which can contain one float value or
// float values. In the first case all borders has the same width
func (b *Builder) parseBorderSizes(fname, field string) (*BorderSizes, error) {

	va, err := b.parseFloats(fname, field, 1, 4)
	if va == nil || err != nil {
		return nil, err
	}
	if len(va) == 1 {
		return &BorderSizes{va[0], va[0], va[0], va[0]}, nil
	}
	return &BorderSizes{va[0], va[1], va[2], va[3]}, nil
}

// parseColor parses a string field which can contain a color name or
// a list of 3 or 4 float values for the color components
func (b *Builder) parseColor(fname, field string) (*math32.Color4, error) {

	// Checks if field is empty
	field = strings.Trim(field, " ")
	if field == "" {
		return nil, nil
	}

	// If string has 1 or 2 fields it must be a color name and optional alpha
	parts := strings.Fields(field)
	if len(parts) == 1 || len(parts) == 2 {
		// First part must be a color name
		if !math32.IsColor(parts[0]) {
			return nil, b.err(fname, fmt.Sprintf("Invalid color name:%s", parts[0]))
		}
		c := math32.ColorName(parts[0])
		c4 := math32.Color4{c.R, c.G, c.B, 1}
		if len(parts) == 2 {
			val, err := strconv.ParseFloat(parts[1], 32)
			if err != nil {
				return nil, b.err(fname, fmt.Sprintf("Invalid float32 value:%s", parts[1]))
			}
			c4.A = float32(val)
		}
		return &c4, nil
	}

	// Accept 3 or 4 floats values
	va, err := b.parseFloats(fname, field, 3, 4)
	if err != nil {
		return nil, err
	}
	if len(va) == 3 {
		return &math32.Color4{va[0], va[1], va[2], 1}, nil
	}
	return &math32.Color4{va[0], va[1], va[2], va[3]}, nil
}

// parseIconNames parses a string with a list of icon names or codepoints and
// returns a string with the icons codepoints encoded in UTF8
func (b *Builder) parseIconNames(fname, field string) (string, error) {

	text := ""
	parts := strings.Fields(field)
	for i := 0; i < len(parts); i++ {
		cp, err := b.parseIconName(fname, parts[i])
		if err != nil {
			return "", err
		}
		text = text + string(cp)
	}
	return text, nil
}

// parseIconName parses a string with an icon name or codepoint in hex
// and returns the icon codepoints value and an error
func (b *Builder) parseIconName(fname, field string) (string, error) {

	// Try name first
	cp := icon.Codepoint(field)
	if cp != "" {
		return cp, nil
	}

	// Try to parse as hex value
	cp2, err := strconv.ParseUint(field, 16, 32)
	if err != nil {
		return "", b.err(fname, fmt.Sprintf("Invalid icon codepoint value/name:%v", field))
	}
	return string(cp2), nil
}

// parseFloats parses a string with a list of floats with the specified size
// and returns a slice. The specified size is 0 any number of floats is allowed.
// The individual values can be separated by spaces or commas
func (b *Builder) parseFloats(fname, field string, min, max int) ([]float32, error) {

	// Checks if field is empty
	field = strings.Trim(field, " ")
	if field == "" {
		return nil, nil
	}

	// Separate individual fields
	var parts []string
	if strings.Index(field, ",") < 0 {
		parts = strings.Fields(field)
	} else {
		parts = strings.Split(field, ",")
	}
	if len(parts) < min || len(parts) > max {
		return nil, b.err(fname, "Invalid number of float32 values")
	}

	// Parse each field value and appends to slice
	var values []float32
	for i := 0; i < len(parts); i++ {
		val, err := strconv.ParseFloat(strings.Trim(parts[i], " "), 32)
		if err != nil {
			return nil, b.err(fname, err.Error())
		}
		values = append(values, float32(val))
	}
	return values, nil
}

// err creates and returns an error for the current object, field name and with the specified message
func (b *Builder) err(fname, msg string) error {

	return fmt.Errorf("Error in object:%s field:%s -> %s", b.objpath.path(), fname, msg)
}

// setupDescTree sets the types of all description tree elements to lower case and
// sets the items "parent" attribute pointing the respective parent description
func (b *Builder) setupDescTree(dp *descPanel) {

	dp.Type = strings.ToLower(dp.Type)
	if dp.Layout != nil {
		dp.Layout.Type = strings.ToLower(dp.Layout.Type)
	}
	for i := 0; i < len(dp.Items); i++ {
		dp.Items[i].parent = dp
		b.setupDescTree(dp.Items[i])
	}
	// Special case for splitter
	if dp.P0 != nil {
		dp.P0.parent = dp
		b.setupDescTree(dp.P0)
	}
	if dp.P1 != nil {
		dp.P1.parent = dp
		b.setupDescTree(dp.P1)
	}
}

// Returns the width and height attributes of the specified descriptor
// if defined or the defaul value (0).
func (b *Builder) size(dp *descPanel) (float32, float32) {

	var width, height float32
	if dp.Width != nil {
		width = *dp.Width
	}
	if dp.Height != nil {
		height = *dp.Height
	}
	return width, height
}

// strStack is a stack of strings
type strStack struct {
	stack []string
}

// clear removes all elements from the stack
func (ss *strStack) clear() {

	ss.stack = []string{}
}

// push pushes a string to the top of the stack
func (ss *strStack) push(v string) {

	ss.stack = append(ss.stack, v)
}

// pop removes and returns the string at the top of the stack.
// Returns an empty string if the stack is empty
func (ss *strStack) pop() string {

	if len(ss.stack) == 0 {
		return ""
	}
	length := len(ss.stack)
	v := ss.stack[length-1]
	ss.stack = ss.stack[:length-1]
	return v
}

// path returns a string composed of all the strings in the
// stack separated by a forward slash.
func (ss *strStack) path() string {

	return strings.Join(ss.stack, "/")
}
