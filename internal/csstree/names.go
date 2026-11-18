package csstree

import "strings"

// KeywordDescriptor describes a keyword or at-rule name (css-tree's
// keyword()).
type KeywordDescriptor struct {
	Basename string // name without vendor prefix
	Name     string // lowercased keyword
	Prefix   string // same as Vendor
	Vendor   string // vendor prefix such as "-webkit-", or ""
	Custom   bool   // custom identifier starting with "--"
}

// PropertyDescriptor describes a property name (css-tree's property()).
type PropertyDescriptor struct {
	Basename string // name without hack and vendor prefix
	Name     string // name without hack; lowercased unless custom
	Hack     string // IE hack prefix: "_", "*", "$", "#", "+", "&", "/" or "//"
	Vendor   string // vendor prefix such as "-webkit-", or ""
	Prefix   string // hack followed by vendor prefix
	Custom   bool   // custom property starting with "--"
}

// IsCustomProperty reports whether str starts with "--".
func IsCustomProperty(str string) bool {
	return isCustomPropertyAt(str, 0)
}

func isCustomPropertyAt(str string, offset int) bool {
	return len(str)-offset >= 2 && str[offset] == '-' && str[offset+1] == '-'
}

// VendorPrefix returns the vendor prefix of str (e.g. "-moz-"), or "".
func VendorPrefix(str string) string {
	return vendorPrefixAt(str, 0)
}

// vendorPrefixAt returns the vendor prefix starting at byte offset: a
// hyphen, a non-hyphen and anything up to and including the next hyphen.
func vendorPrefixAt(str string, offset int) string {
	if UTF16Len(str[offset:]) >= 3 && str[offset] == '-' && str[offset+1] != '-' {
		if i := strings.IndexByte(str[offset+2:], '-'); i != -1 {
			return str[offset : offset+2+i+1]
		}
	}
	return ""
}

// Keyword resolves a keyword or at-rule name into its parts.
func Keyword(keyword string) KeywordDescriptor {
	name := LowerCase(keyword)
	custom := IsCustomProperty(name)
	vendor := ""
	if !custom {
		vendor = vendorPrefixAt(name, 0)
	}
	return KeywordDescriptor{
		Basename: name[len(vendor):],
		Name:     name,
		Prefix:   vendor,
		Vendor:   vendor,
		Custom:   custom,
	}
}

// Property resolves a property name into its hack, vendor prefix and base
// name.
func Property(property string) PropertyDescriptor {
	hack := ""
	if property != "" {
		switch property[0] {
		case '/':
			hack = "/"
			if len(property) > 1 && property[1] == '/' {
				hack = "//"
			}
		case '_', '*', '$', '#', '+', '&':
			hack = property[:1]
		}
	}

	name := property
	custom := isCustomPropertyAt(name, len(hack))
	vendor := ""
	if !custom {
		name = LowerCase(name)
		vendor = vendorPrefixAt(name, len(hack))
	}
	prefix := name[:len(hack)+len(vendor)]

	return PropertyDescriptor{
		Basename: name[len(prefix):],
		Name:     name[len(hack):],
		Hack:     hack,
		Vendor:   vendor,
		Prefix:   prefix,
		Custom:   custom,
	}
}
