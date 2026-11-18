package restructure

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/css/csso-go/internal/csso/jscompat"
	"github.com/css/csso-go/internal/csstree"
)

// fingerprintID numbers the fingerprints of values that contain unparsed
// text, so that they never compare equal. Like csso's module-level counter
// it keeps counting across compressions.
var fingerprintID atomic.Int64

// dontRestructure lists properties whose repeated declarations are all kept
// (csso issue #50).
var dontRestructure = map[string]bool{"src": true}

// dontMixValue matches the keywords that must not be merged with other
// values of the same property since browser support differs.
var dontMixValue = map[string]*regexp.Regexp{
	"display":    regexp.MustCompile(`(?i)table|ruby|flex|-(flex)?box$|grid|contents|run-in`),
	"text-align": regexp.MustCompile(`(?i)^(start|end|match-parent|justify-all)$`),
}

// safeValues lists the keywords of a property that may be merged with each
// other; any other keyword is special.
var safeValues = map[string][]string{
	"cursor": {
		"auto", "crosshair", "default", "move", "text", "wait", "help",
		"n-resize", "e-resize", "s-resize", "w-resize",
		"ne-resize", "nw-resize", "se-resize", "sw-resize",
		"pointer", "progress", "not-allowed", "no-drop", "vertical-text", "all-scroll",
		"col-resize", "row-resize",
	},
	"overflow": {"hidden", "visible", "scroll", "auto"},
	"position": {"static", "relative", "absolute", "fixed"},
}

// needlessTable maps a property to the shorthands that make it redundant
// when they follow with the same fingerprint.
var needlessTable = map[string][]string{
	"border-width":        {"border"},
	"border-style":        {"border"},
	"border-color":        {"border"},
	"border-top":          {"border"},
	"border-right":        {"border"},
	"border-bottom":       {"border"},
	"border-left":         {"border"},
	"border-top-width":    {"border-top", "border-width", "border"},
	"border-right-width":  {"border-right", "border-width", "border"},
	"border-bottom-width": {"border-bottom", "border-width", "border"},
	"border-left-width":   {"border-left", "border-width", "border"},
	"border-top-style":    {"border-top", "border-style", "border"},
	"border-right-style":  {"border-right", "border-style", "border"},
	"border-bottom-style": {"border-bottom", "border-style", "border"},
	"border-left-style":   {"border-left", "border-style", "border"},
	"border-top-color":    {"border-top", "border-color", "border"},
	"border-right-color":  {"border-right", "border-color", "border"},
	"border-bottom-color": {"border-bottom", "border-color", "border"},
	"border-left-color":   {"border-left", "border-color", "border"},
	"margin-top":          {"margin"},
	"margin-right":        {"margin"},
	"margin-bottom":       {"margin"},
	"margin-left":         {"margin"},
	"padding-top":         {"padding"},
	"padding-right":       {"padding"},
	"padding-bottom":      {"padding"},
	"padding-left":        {"padding"},
	"font-style":          {"font"},
	"font-variant":        {"font"},
	"font-weight":         {"font"},
	"font-size":           {"font"},
	"font-family":         {"font"},
	"list-style-type":     {"list-style"},
	"list-style-position": {"list-style"},
	"list-style-image":    {"list-style"},
}

// ieHack matches an IE hack (`\0` or `\9`) in a name or unit.
var ieHack = regexp.MustCompile(`\\[09]`)

// propertyFingerprint returns the key under which declarations of property
// propertyName override each other: the name plus a fingerprint of the
// value's features that browsers may not support alike (special keywords,
// functions and units, IE hacks, vendor prefixes). fingerprints caches the
// value fingerprint by declaration id.
func propertyFingerprint(propertyName string, declaration *csstree.Declaration, fingerprints map[int]string) string {
	realName := csstree.Property(propertyName).Basename

	if realName == "background" {
		return propertyName + ":" + csstree.Generate(declaration.Value)
	}

	fingerprint, ok := fingerprints[declaration.ID]
	if !ok || fingerprint == "" {
		switch value := declaration.Value.(type) {
		case *csstree.Value:
			fingerprint = valueFingerprint(realName, value)
		case *csstree.Raw:
			fingerprint = "!" + value.Value
		default:
			fingerprint = csstree.Generate(declaration.Value)
		}
		fingerprints[declaration.ID] = fingerprint
	}

	return propertyName + fingerprint
}

// valueFingerprint is the fingerprint of a parsed value of property
// realName.
func valueFingerprint(realName string, value *csstree.Value) string {
	special := make(map[string]bool)
	vendorID := ""
	iehack := ""
	raw := false

	var walk func(node csstree.Node, _ *csstree.ListItem)
	walk = func(node csstree.Node, _ *csstree.ListItem) {
		switch n := node.(type) {
		case *csstree.Value:
			n.Children.ForEach(walk)
		case *csstree.Brackets:
			n.Children.ForEach(walk)
		case *csstree.Parentheses:
			n.Children.ForEach(walk)

		case *csstree.Raw:
			raw = true

		case *csstree.Identifier:
			name := n.Name
			if vendorID == "" {
				vendorID = csstree.Keyword(name).Vendor
			}
			if hack := ieHack.FindString(name); hack != "" {
				iehack = hack
			}
			if safe, ok := safeValues[realName]; ok {
				if !slices.Contains(safe, name) {
					special[name] = true
				}
			} else if dontMix, ok := dontMixValue[realName]; ok {
				if dontMix.MatchString(name) {
					special[name] = true
				}
			}

		case *csstree.Function:
			name := n.Name
			if vendorID == "" {
				vendorID = csstree.Keyword(name).Vendor
			}
			if name == "rect" {
				// rect(<top>, <right>, <bottom>, <left>) and the backwards
				// compatible rect(<top> <right> <bottom> <left>) don't mix
				hasComma := n.Children.Some(func(child csstree.Node, _ *csstree.ListItem) bool {
					op, ok := child.(*csstree.Operator)
					return ok && op.Value == ","
				})
				if !hasComma {
					name = "rect-backward"
				}
			}
			special[name+"()"] = true

			// check nested tokens too
			n.Children.ForEach(walk)

		case *csstree.Dimension:
			if hack := ieHack.FindString(n.Unit); hack != "" {
				iehack = hack
			}
			switch n.Unit {
			// rem is not supported until IE11 and the viewport units are too
			// buggy across browsers ("vm" is IE9's "vmin")
			case "rem", "vw", "vh", "vmin", "vmax", "vm":
				special[n.Unit] = true
			}
		}
	}
	value.Children.ForEach(walk)

	if raw {
		return "!" + strconv.FormatInt(fingerprintID.Add(1), 10)
	}

	keys := make([]string, 0, len(special))
	for key := range special {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, jscompat.Compare)
	return "!" + strings.Join(keys, ",") + "|" + iehack + vendorID
}

// declarationRef locates a declaration that may override others.
type declarationRef struct {
	block *csstree.List
	item  *csstree.ListItem
}

func (r *declarationRef) declaration() *csstree.Declaration {
	return r.item.Data.(*csstree.Declaration)
}

// needless returns a following shorthand that makes declaration redundant,
// or nil.
func needless(props map[string]*declarationRef, declaration *csstree.Declaration, fingerprints map[int]string) *declarationRef {
	property := csstree.Property(declaration.Property)
	for _, entry := range needlessTable[property.Basename] {
		ppre := propertyFingerprint(property.Prefix+entry, declaration, fingerprints)
		if prev := props[ppre]; prev != nil && (declaration.Important == "" || prev.declaration().Important != "") {
			return prev
		}
	}
	return nil
}

// restructRule removes the declarations of a rule that a later declaration
// of the same property and fingerprint (or a later shorthand) overrides,
// within rules of the same selector; a rule left empty is removed.
func restructRule(rule *csstree.Rule, item *csstree.ListItem, list *csstree.List, props map[string]*declarationRef, fingerprints map[int]string) error {
	declarations := rule.Block.Children
	var err error

	declarations.ForEachRight(func(node csstree.Node, declarationItem *csstree.ListItem) {
		if err != nil {
			return
		}
		declaration, ok := node.(*csstree.Declaration)
		if !ok {
			// csso reads the property name of every block item
			err = jscompat.ReadPropertyError("undefined", "0")
			return
		}

		property := declaration.Property
		fingerprint := propertyFingerprint(property, declaration, fingerprints)

		if prev := props[fingerprint]; prev != nil && !dontRestructure[property] {
			if declaration.Important != "" && prev.declaration().Important == "" {
				props[fingerprint] = &declarationRef{block: declarations, item: declarationItem}
				prev.block.Remove(prev.item)
			} else {
				declarations.Remove(declarationItem)
			}
			return
		}

		if needless(props, declaration, fingerprints) != nil {
			declarations.Remove(declarationItem)
			return
		}

		declaration.Fingerprint = fingerprint
		props[fingerprint] = &declarationRef{block: declarations, item: declarationItem}
	})
	if err != nil {
		return err
	}

	if declarations.IsEmpty() {
		list.Remove(item)
	}
	return nil
}

// restructBlock removes overridden declarations across the rules with the
// same selector in each block (csso restructure/6-restructBlock.js).
func restructBlock(ast *csstree.StyleSheet) error {
	stylesheetMap := make(map[int]map[string]map[string]*declarationRef)
	fingerprints := make(map[int]string)
	var err error

	csstree.Walk(ast, csstree.WalkOptions{
		Visit:   "Rule",
		Reverse: true,
		Enter: func(ctx *csstree.WalkContext, node csstree.Node, item *csstree.ListItem, list *csstree.List) csstree.WalkAction {
			rule := node.(*csstree.Rule)
			ruleID := rule.PseudoSignature + "|" + selector(selectorList(rule).First()).ID

			ruleMap, ok := stylesheetMap[containerID(ctx)]
			if !ok {
				ruleMap = make(map[string]map[string]*declarationRef)
				stylesheetMap[containerID(ctx)] = ruleMap
			}

			props, ok := ruleMap[ruleID]
			if !ok {
				props = make(map[string]*declarationRef)
				ruleMap[ruleID] = props
			}

			if err = restructRule(rule, item, list, props, fingerprints); err != nil {
				return csstree.WalkBreak
			}
			return csstree.WalkContinue
		},
	})
	return err
}
