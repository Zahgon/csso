package replace

import (
	"regexp"

	"github.com/css/csso-go/internal/csso/nodes"
	"github.com/css/csso-go/internal/csstree"
)

// omitPlusSign splits a number into its sign (minus only), integer digits
// without leading zeros and fraction without trailing zeros (csso
// replace/Number.js OMIT_PLUSSIGN).
var omitPlusSign = regexp.MustCompile(`^(?:\+|(-))?0*(\d*)(?:\.0*|(\.\d*?)0*)?$`)

// PackNumber returns the shortest form of a number: `+` and leading zeros
// dropped, trailing fraction zeros dropped and `0.5` written `.5`; a number
// in exponent form is kept as is.
func PackNumber(value string) string {
	value = omitPlusSign.ReplaceAllString(value, "${1}${2}${3}")
	if value == "" || value == "-" {
		return "0"
	}
	return value
}

// number packs the value of a Number node.
func number(node *csstree.Number) {
	node.Value = PackNumber(node.Value)
}

// mathFunctions are the functions in which zero lengths keep their unit
// (csso issue #222).
var mathFunctions = map[string]bool{"calc": true, "min": true, "max": true, "clamp": true}

// lengthUnits are the units of the lengths whose zero csso writes unitless.
var lengthUnits = map[string]bool{
	// absolute length units
	"px": true, "mm": true, "cm": true, "in": true, "pt": true, "pc": true,
	// relative length units
	"em": true, "ex": true, "ch": true, "rem": true,
	// viewport-percentage lengths
	"vh": true, "vw": true, "vmin": true, "vmax": true, "vm": true,
}

// dimension packs the number of a Dimension and drops the unit of a zero
// length in a declaration value, except in flex shorthands (they break in
// IE10/11, csso issues #200 and #362) and math functions.
func dimension(ctx *csstree.WalkContext, node *csstree.Dimension, item *csstree.ListItem) error {
	value := PackNumber(node.Value)
	node.Value = value

	if value != "0" || ctx.Declaration == nil || ctx.AtrulePrelude != nil {
		return nil
	}
	if !lengthUnits[csstree.LowerCase(node.Unit)] {
		return nil
	}
	if property := ctx.Declaration.Property; property == "-ms-flex" || property == "flex" {
		return nil
	}
	if ctx.Function != nil && mathFunctions[nodes.FunctionName(ctx.Function)] {
		return nil
	}
	return setData(item, &csstree.Number{Base: node.Base, Value: value})
}

// percentageKeepUnit lists the properties whose zero percentage keeps its
// unit: sizes (clean-css issue #957) and flex shorthands (csso issues #200,
// #362 and #410).
var percentageKeepUnit = map[string]bool{
	"width": true, "min-width": true, "max-width": true,
	"height": true, "min-height": true, "max-height": true,
	"flex": true, "-ms-flex": true,
}

// percentage packs the number of a Percentage and writes a zero percentage
// as `0` where the declaration still matches its syntax with a length there.
func percentage(ctx *csstree.WalkContext, node *csstree.Percentage, item *csstree.ListItem) error {
	node.Value = PackNumber(node.Value)

	if node.Value != "0" || ctx.Declaration == nil || percentageKeepUnit[ctx.Declaration.Property] {
		return nil
	}

	zero := &csstree.Number{Base: node.Base, Value: node.Value}
	if err := setData(item, zero); err != nil {
		return err
	}
	if !matchesType(ctx.Declaration, zero, "length") {
		// the unitless zero changes the meaning: roll back
		item.Data = node
	}
	return nil
}
