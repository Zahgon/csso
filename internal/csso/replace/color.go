package replace

import (
	"math"
	"regexp"
	"strconv"
	"unicode/utf16"

	"github.com/css/csso-go/internal/csso/jscompat"
	"github.com/css/csso-go/internal/csso/nodes"
	"github.com/css/csso-go/internal/csstree"
)

// argKind is the kind of a color function argument (csso's arg.type).
type argKind int

const (
	argNumber argKind = iota
	argPercentage
	argAngle
	argAlpha
)

type colorArg struct {
	kind  argKind
	value float64
}

// gradientScope matches the functions in which a fully transparent color is
// not replaced by `transparent`, since that may change the color transition.
var gradientScope = regexp.MustCompile(`(?i)^(?:to|from|color-stop)$|gradient$`)

func hueToRgb(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	if t < 1.0/6 {
		return p + float64(float64((q-p)*6)*t)
	}
	if t < 1.0/2 {
		return q
	}
	if t < 2.0/3 {
		return p + float64(float64((q-p)*(2.0/3-t))*6)
	}
	return p
}

// hslToRgb converts hue, saturation and lightness (all in [0..1]) to
// rounded red, green and blue channels, passing alpha through.
func hslToRgb(h, s, l, a float64) [4]float64 {
	var r, g, b float64

	if s == 0 {
		r, g, b = l, l, l // achromatic
	} else {
		var q float64
		if l < 0.5 {
			q = float64(l * (1 + s))
		} else {
			q = l + s - float64(l*s)
		}
		p := float64(2*l) - q

		r = hueToRgb(p, q, h+1.0/3)
		g = hueToRgb(p, q, h)
		b = hueToRgb(p, q, h-1.0/3)
	}

	return [4]float64{
		math.Round(float64(r * 255)),
		math.Round(float64(g * 255)),
		math.Round(float64(b * 255)),
		a,
	}
}

func toHex(value float64) string {
	hex := strconv.FormatInt(int64(value), 16)
	if len(hex) == 1 {
		return "0" + hex
	}
	return hex
}

// parseFunctionArgs reads the comma separated arguments of rgb(), rgba(),
// hsl() or hsla() and normalizes them: channels to [0..255], hue, saturation
// and lightness to [0..1] and alpha to [0..1]. It returns nil for anything
// it does not understand: other tokens, a wrong argument count or mixed
// argument types.
func parseFunctionArgs(functionArgs *csstree.List, count int, rgb bool) []float64 {
	var args []colorArg
	wasValue := false

	for cursor := functionArgs.Head; cursor != nil; cursor = cursor.Next {
		switch n := cursor.Data.(type) {
		case *csstree.Number, *csstree.Percentage:
			if wasValue {
				return nil
			}
			wasValue = true
			arg := colorArg{kind: argNumber}
			text := ""
			if num, ok := n.(*csstree.Number); ok {
				text = num.Value
			} else {
				arg.kind = argPercentage
				text = n.(*csstree.Percentage).Value
			}
			arg.value = jsNumber(text)
			args = append(args, arg)

		case *csstree.Operator:
			if n.Value == "," {
				if !wasValue {
					return nil
				}
				wasValue = false
			} else if wasValue || n.Value != "+" {
				return nil
			}

		default:
			// something we couldn't understand
			return nil
		}
	}

	if len(args) != count {
		return nil
	}

	if len(args) == 4 {
		if args[3].kind != argNumber {
			// the alpha value should be a number
			return nil
		}
		args[3].kind = argAlpha
	}

	if rgb {
		// numbers and percentages can't be mixed
		if args[0].kind != args[1].kind || args[0].kind != args[2].kind {
			return nil
		}
	} else {
		// hsl takes a number, a percentage and a percentage
		if args[0].kind != argNumber || args[1].kind != argPercentage || args[2].kind != argPercentage {
			return nil
		}
		args[0].kind = argAngle
	}

	values := make([]float64, len(args))
	for i, arg := range args {
		value := math.Max(0, arg.value)

		switch arg.kind {
		case argNumber:
			// fit value to [0..255] range
			value = math.Round(math.Min(value, 255))
		case argPercentage:
			// convert 0..100% to [0..1] range, or [0..255] for rgb
			value = math.Min(value, 100) / 100
			if rgb {
				value = math.Round(float64(255 * value))
			}
		case argAngle:
			// fit value to [0..360) range and scale to [0..1)
			value = math.Mod(math.Mod(value, 360)+360, 360) / 360
		case argAlpha:
			// fit value to [0..1] range
			value = math.Min(value, 1)
		}
		values[i] = value
	}
	return values
}

// jsNumber converts the text of a Number or Percentage node to a number as
// JavaScript's Number() does; css-tree only produces decimal number text.
func jsNumber(text string) float64 {
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return math.NaN()
	}
	return f
}

// colorFunction shortens rgb(), rgba(), hsl() and hsla(): fully transparent
// colors become `transparent`, translucent ones get normalized arguments
// and opaque ones become a hex color.
func colorFunction(ctx *csstree.WalkContext, node *csstree.Function, item *csstree.ListItem) error {
	functionName := node.Name
	var args []float64

	if functionName == "rgba" || functionName == "hsla" {
		args = parseFunctionArgs(node.Children, 4, functionName == "rgba")
		if args == nil {
			return nil
		}

		if functionName == "hsla" {
			converted := hslToRgb(args[0], args[1], args[2], args[3])
			args = converted[:]
			node.Name = "rgba"
		}

		if args[3] == 0 {
			// rgba(0, 0, 0, 0) is always `transparent`; other fully
			// transparent colors keep their channels in gradients since
			// they affect the color transition
			scopeFunctionName := "null"
			if ctx.Function != nil {
				scopeFunctionName = nodes.FunctionName(ctx.Function)
			}
			if (args[0] == 0 && args[1] == 0 && args[2] == 0) || !gradientScope.MatchString(scopeFunctionName) {
				return setData(item, &csstree.Identifier{Base: node.Base, Name: "transparent"})
			}
		}

		if args[3] != 1 {
			// write the normalized/interpolated argument values
			node.Children.ForEach(func(child csstree.Node, childItem *csstree.ListItem) {
				if op, ok := child.(*csstree.Operator); ok {
					if op.Value != "," {
						node.Children.Remove(childItem)
					}
					return
				}
				childItem.Data = &csstree.Number{
					Base:  csstree.Base{Location: child.Loc()},
					Value: PackNumber(jscompat.NumberToString(args[0])),
				}
				args = args[1:]
			})
			return nil
		}

		// opaque: continue as rgb(), e.g. rgba(255, 0, 0, 1) -> rgb(255, 0, 0)
		functionName = "rgb"
	}

	if functionName == "hsl" {
		if args == nil {
			args = parseFunctionArgs(node.Children, 3, false)
		}
		if args == nil {
			return nil
		}
		converted := hslToRgb(args[0], args[1], args[2], 0)
		args = converted[:]
		functionName = "rgb"
	}

	if functionName == "rgb" {
		if args == nil {
			args = parseFunctionArgs(node.Children, 3, true)
		}
		if args == nil {
			return nil
		}
		hash := &csstree.Hash{Base: node.Base, Value: toHex(args[0]) + toHex(args[1]) + toHex(args[2])}
		if err := setData(item, hash); err != nil {
			return err
		}
		return hexColor(hash, item)
	}
	return nil
}

// identColor replaces a color keyword by a shorter hex value, or lowercases
// it (writing `grey` as `gray`), where the declaration syntax matches it as
// a color.
func identColor(ctx *csstree.WalkContext, node *csstree.Identifier, item *csstree.ListItem) error {
	if ctx.Declaration == nil {
		return nil
	}

	color := csstree.LowerCase(node.Name)
	hex, ok := nameToHex[color]
	if !ok || !matchesType(ctx.Declaration, node, "color") {
		return nil
	}

	if len(hex)+1 <= csstree.UTF16Len(color) {
		// replace for the shorter hex value
		return setData(item, &csstree.Hash{Base: node.Base, Value: hex})
	}

	// keep a consistent spelling
	if color == "grey" {
		color = "gray"
	}
	node.Name = color
	return nil
}

// hexColor lowercases a hex color, writes #aabbcc as #abc and replaces it by
// a color name where that is not longer.
func hexColor(node *csstree.Hash, item *csstree.ListItem) error {
	color := csstree.LowerCase(node.Value)

	// #112233 -> #123 (comparing UTF-16 code units, as csso does)
	if units := utf16.Encode([]rune(color)); len(units) == 6 &&
		units[0] == units[1] && units[2] == units[3] && units[4] == units[5] {
		color = string(utf16.Decode([]uint16{units[0], units[2], units[4]}))
	}

	if name, ok := hexToName[color]; ok {
		return setData(item, &csstree.Identifier{Base: node.Base, Name: name})
	}
	node.Value = color
	return nil
}

// nameToHex maps the named colors of CSS Color 3 to their hex value
// (csso replace/color.js NAME_TO_HEX).
var nameToHex = map[string]string{
	"aliceblue":            "f0f8ff",
	"antiquewhite":         "faebd7",
	"aqua":                 "0ff",
	"aquamarine":           "7fffd4",
	"azure":                "f0ffff",
	"beige":                "f5f5dc",
	"bisque":               "ffe4c4",
	"black":                "000",
	"blanchedalmond":       "ffebcd",
	"blue":                 "00f",
	"blueviolet":           "8a2be2",
	"brown":                "a52a2a",
	"burlywood":            "deb887",
	"cadetblue":            "5f9ea0",
	"chartreuse":           "7fff00",
	"chocolate":            "d2691e",
	"coral":                "ff7f50",
	"cornflowerblue":       "6495ed",
	"cornsilk":             "fff8dc",
	"crimson":              "dc143c",
	"cyan":                 "0ff",
	"darkblue":             "00008b",
	"darkcyan":             "008b8b",
	"darkgoldenrod":        "b8860b",
	"darkgray":             "a9a9a9",
	"darkgrey":             "a9a9a9",
	"darkgreen":            "006400",
	"darkkhaki":            "bdb76b",
	"darkmagenta":          "8b008b",
	"darkolivegreen":       "556b2f",
	"darkorange":           "ff8c00",
	"darkorchid":           "9932cc",
	"darkred":              "8b0000",
	"darksalmon":           "e9967a",
	"darkseagreen":         "8fbc8f",
	"darkslateblue":        "483d8b",
	"darkslategray":        "2f4f4f",
	"darkslategrey":        "2f4f4f",
	"darkturquoise":        "00ced1",
	"darkviolet":           "9400d3",
	"deeppink":             "ff1493",
	"deepskyblue":          "00bfff",
	"dimgray":              "696969",
	"dimgrey":              "696969",
	"dodgerblue":           "1e90ff",
	"firebrick":            "b22222",
	"floralwhite":          "fffaf0",
	"forestgreen":          "228b22",
	"fuchsia":              "f0f",
	"gainsboro":            "dcdcdc",
	"ghostwhite":           "f8f8ff",
	"gold":                 "ffd700",
	"goldenrod":            "daa520",
	"gray":                 "808080",
	"grey":                 "808080",
	"green":                "008000",
	"greenyellow":          "adff2f",
	"honeydew":             "f0fff0",
	"hotpink":              "ff69b4",
	"indianred":            "cd5c5c",
	"indigo":               "4b0082",
	"ivory":                "fffff0",
	"khaki":                "f0e68c",
	"lavender":             "e6e6fa",
	"lavenderblush":        "fff0f5",
	"lawngreen":            "7cfc00",
	"lemonchiffon":         "fffacd",
	"lightblue":            "add8e6",
	"lightcoral":           "f08080",
	"lightcyan":            "e0ffff",
	"lightgoldenrodyellow": "fafad2",
	"lightgray":            "d3d3d3",
	"lightgrey":            "d3d3d3",
	"lightgreen":           "90ee90",
	"lightpink":            "ffb6c1",
	"lightsalmon":          "ffa07a",
	"lightseagreen":        "20b2aa",
	"lightskyblue":         "87cefa",
	"lightslategray":       "789",
	"lightslategrey":       "789",
	"lightsteelblue":       "b0c4de",
	"lightyellow":          "ffffe0",
	"lime":                 "0f0",
	"limegreen":            "32cd32",
	"linen":                "faf0e6",
	"magenta":              "f0f",
	"maroon":               "800000",
	"mediumaquamarine":     "66cdaa",
	"mediumblue":           "0000cd",
	"mediumorchid":         "ba55d3",
	"mediumpurple":         "9370db",
	"mediumseagreen":       "3cb371",
	"mediumslateblue":      "7b68ee",
	"mediumspringgreen":    "00fa9a",
	"mediumturquoise":      "48d1cc",
	"mediumvioletred":      "c71585",
	"midnightblue":         "191970",
	"mintcream":            "f5fffa",
	"mistyrose":            "ffe4e1",
	"moccasin":             "ffe4b5",
	"navajowhite":          "ffdead",
	"navy":                 "000080",
	"oldlace":              "fdf5e6",
	"olive":                "808000",
	"olivedrab":            "6b8e23",
	"orange":               "ffa500",
	"orangered":            "ff4500",
	"orchid":               "da70d6",
	"palegoldenrod":        "eee8aa",
	"palegreen":            "98fb98",
	"paleturquoise":        "afeeee",
	"palevioletred":        "db7093",
	"papayawhip":           "ffefd5",
	"peachpuff":            "ffdab9",
	"peru":                 "cd853f",
	"pink":                 "ffc0cb",
	"plum":                 "dda0dd",
	"powderblue":           "b0e0e6",
	"purple":               "800080",
	"rebeccapurple":        "639",
	"red":                  "f00",
	"rosybrown":            "bc8f8f",
	"royalblue":            "4169e1",
	"saddlebrown":          "8b4513",
	"salmon":               "fa8072",
	"sandybrown":           "f4a460",
	"seagreen":             "2e8b57",
	"seashell":             "fff5ee",
	"sienna":               "a0522d",
	"silver":               "c0c0c0",
	"skyblue":              "87ceeb",
	"slateblue":            "6a5acd",
	"slategray":            "708090",
	"slategrey":            "708090",
	"snow":                 "fffafa",
	"springgreen":          "00ff7f",
	"steelblue":            "4682b4",
	"tan":                  "d2b48c",
	"teal":                 "008080",
	"thistle":              "d8bfd8",
	"tomato":               "ff6347",
	"turquoise":            "40e0d0",
	"violet":               "ee82ee",
	"wheat":                "f5deb3",
	"white":                "fff",
	"whitesmoke":           "f5f5f5",
	"yellow":               "ff0",
	"yellowgreen":          "9acd32",
}

// hexToName maps hex values to a color name not longer than the hex form
// (csso replace/color.js HEX_TO_NAME).
var hexToName = map[string]string{
	"800000": "maroon",
	"800080": "purple",
	"808000": "olive",
	"808080": "gray",
	"00ffff": "cyan",
	"f0ffff": "azure",
	"f5f5dc": "beige",
	"ffe4c4": "bisque",
	"000000": "black",
	"0000ff": "blue",
	"a52a2a": "brown",
	"ff7f50": "coral",
	"ffd700": "gold",
	"008000": "green",
	"4b0082": "indigo",
	"fffff0": "ivory",
	"f0e68c": "khaki",
	"00ff00": "lime",
	"faf0e6": "linen",
	"000080": "navy",
	"ffa500": "orange",
	"da70d6": "orchid",
	"cd853f": "peru",
	"ffc0cb": "pink",
	"dda0dd": "plum",
	"f00":    "red",
	"ff0000": "red",
	"fa8072": "salmon",
	"a0522d": "sienna",
	"c0c0c0": "silver",
	"fffafa": "snow",
	"d2b48c": "tan",
	"008080": "teal",
	"ff6347": "tomato",
	"ee82ee": "violet",
	"f5deb3": "wheat",
	"ffffff": "white",
	"ffff00": "yellow",
}
