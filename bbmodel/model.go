// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package bbmodel

import "github.com/google/uuid"

type Model struct {
	Meta       Meta        `json:"meta"`
	Name       string      `json:"name"`
	Resolution *Resolution `json:"resolution,omitempty"`
	Elements   []Element   `json:"elements"`
	Groups     []Group     `json:"groups"`
	Outliner   []any       `json:"outliner"`
	Textures   []Texture   `json:"textures"`
	Animations []Animation `json:"animations,omitempty"`
}

type Meta struct {
	FormatVersion string `json:"format_version"`
	ModelFormat   string `json:"model_format"`
	BoxUV         bool   `json:"box_uv"`
}

type Resolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Element struct {
	Name          string          `json:"name"`
	BoxUV         bool            `json:"box_uv"`
	RenderOrder   string          `json:"render_order"`
	Locked        bool            `json:"locked"`
	From          [3]float32      `json:"from"`
	To            [3]float32      `json:"to"`
	Autouv        int             `json:"autouv"`
	Color         int             `json:"color"`
	Origin        [3]float32      `json:"origin"`
	Faces         map[string]Face `json:"faces"`
	Type          string          `json:"type"`
	UUID          string          `json:"uuid"`
	LightEmission *int            `json:"light_emission,omitempty"`
	Shade         *bool           `json:"shade,omitempty"`
}

type Face struct {
	UV      [4]float32 `json:"uv"`
	Texture *int       `json:"texture,omitempty"`
}

type Group struct {
	UUID            string     `json:"uuid"`
	Export          bool       `json:"export"`
	Locked          bool       `json:"locked"`
	Origin          [3]float32 `json:"origin"`
	Rotation        [3]float32 `json:"rotation"`
	Color           int        `json:"color"`
	Name            string     `json:"name"`
	Children        []any      `json:"children"`
	Reset           bool       `json:"reset"`
	Shade           bool       `json:"shade"`
	MirrorUV        bool       `json:"mirror_uv"`
	Selected        bool       `json:"selected"`
	Visibility      bool       `json:"visibility"`
	Autouv          int        `json:"autouv"`
	IsOpen          bool       `json:"isOpen"`
	PrimarySelected bool       `json:"primary_selected"`
}

type OutlinerNode struct {
	UUID     string `json:"uuid"`
	IsOpen   bool   `json:"isOpen"`
	Children []any  `json:"children"`
}

type Texture struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	UVWidth     int    `json:"uv_width"`
	UVHeight    int    `json:"uv_height"`
	Particle    bool   `json:"particle"`
	RenderMode  string `json:"render_mode"`
	RenderSides string `json:"render_sides"`
	Visible     bool   `json:"visible"`
	Internal    bool   `json:"internal"`
	Saved       bool   `json:"saved"`
	UUID        string `json:"uuid"`
	Source      string `json:"source"`
}

func NewUUID() string {
	return uuid.New().String()
}

func NewModel(name string) *Model {
	return &Model{
		Meta: Meta{
			FormatVersion: "5.0",
			ModelFormat:   "free",
			BoxUV:         false,
		},
		Name:     name,
		Elements: []Element{},
		Groups:   []Group{},
		Outliner: []any{},
		Textures: []Texture{},
	}
}

// Animation Blockbench 动画
type Animation struct {
	UUID      string              `json:"uuid"`
	Name      string              `json:"name"`
	Loop      string              `json:"loop"` // "once", "loop", "hold"
	Override  bool                `json:"override"`
	Length    float64             `json:"length"`
	Snapping  int                 `json:"snapping"`
	Animators map[string]Animator `json:"animators"`
}

// Animator 骨骼动画器
type Animator struct {
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Keyframes []Keyframe `json:"keyframes"`
}

// Keyframe 关键帧
type Keyframe struct {
	Channel       string      `json:"channel"`
	DataPoints    []DataPoint `json:"data_points"`
	UUID          string      `json:"uuid"`
	Time          float64     `json:"time"`
	Color         int         `json:"color"`
	Interpolation string      `json:"interpolation"` // "linear", "step", "catmullrom", "bezier"
}

// DataPoint 关键帧数据点（x/y/z 必须是字符串）
type DataPoint struct {
	X string `json:"x"`
	Y string `json:"y"`
	Z string `json:"z"`
}
