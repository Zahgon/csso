package csso

import (
	"strconv"
	"strings"

	"github.com/css/csso-go/internal/csso/clean"
	"github.com/css/csso-go/internal/csso/replace"
	"github.com/css/csso-go/internal/csso/restructure"
	"github.com/css/csso-go/internal/csso/usage"
	"github.com/css/csso-go/internal/csstree"
)

// compressor holds the settings of one compression (csso compress.js).
type compressor struct {
	restructure     bool
	forceMediaMerge bool
	usage           *usage.Index
	logger          func(title string, ast csstree.Node)
	// detached is the stand-in block of a root that is not a Block.
	detached *csstree.Block
}

func newCompressor(opts Options) (*compressor, error) {
	c := &compressor{
		restructure:     opts.restructure(),
		forceMediaMerge: opts.ForceMediaMerge,
		logger:          func(string, csstree.Node) {},
	}
	if opts.Logger != nil {
		c.logger = func(title string, ast csstree.Node) {
			if ast == nil {
				opts.Logger(title, nil)
				return
			}
			opts.Logger(title, &AST{node: ast})
		}
	}
	if opts.Usage != nil {
		index, err := opts.Usage.index()
		if err != nil {
			return nil, err
		}
		c.usage = index
	}
	return c, nil
}

// compress minifies the tree rooted at ast and returns the compressed root:
// ast itself (changed in place) or, with Options.Clone, a compressed copy.
//
// A stylesheet is processed in chunks separated by the exclamation comments
// that are kept. Any other root is wrapped, as csso does, as the block of a
// dummy rule `x{}` in a stylesheet of its own.
func compress(ast csstree.Node, opts Options) (csstree.Node, error) {
	c, err := newCompressor(opts)
	if err != nil {
		return nil, err
	}

	if ast == nil {
		ast = &csstree.StyleSheet{Children: csstree.NewList()}
	}
	if opts.Clone {
		ast = csstree.Clone(ast)
	}

	output := csstree.NewList()
	var input *csstree.List

	switch root := ast.(type) {
	case *csstree.StyleSheet:
		input = root.Children
		root.Children = output
	case *csstree.Block:
		input = wrapBlock(root)
	default:
		children := csstree.Children(root)
		if children == nil {
			return ast, c.compressChildless(root)
		}
		// css-tree's walker does not treat the root as a block; the
		// stand-in block shares the root's children
		c.detached = &csstree.Block{Children: children}
		input = wrapBlock(c.detached)
	}

	specialComments := opts.Comments
	firstAtrulesAllowed := true

	for chunkNum := 1; ; chunkNum++ {
		comment, chunk := readChunk(input, specialComments != CommentsNone)
		if err := c.compressChunk(chunk, firstAtrulesAllowed, chunkNum); err != nil {
			return nil, convertError(err)
		}
		chunkChildren := chunk.Children

		if comment != nil {
			// separate the comment from the content around it
			if !output.IsEmpty() {
				output.AppendData(&csstree.Raw{Value: "\n"})
			}
			output.AppendData(comment)
			if !chunkChildren.IsEmpty() {
				output.AppendData(&csstree.Raw{Value: "\n"})
			}
		}

		if firstAtrulesAllowed && !chunkChildren.IsEmpty() {
			if last, ok := chunkChildren.Last().(*csstree.Atrule); !ok || (last.Name != "import" && last.Name != "charset") {
				firstAtrulesAllowed = false
			}
		}

		if specialComments != CommentsExclamation {
			specialComments = CommentsNone
		}

		output.AppendList(chunkChildren)

		if input.IsEmpty() {
			return ast, nil
		}
	}
}

// wrapBlock returns a list holding the rule `x{}` with block as its block.
func wrapBlock(block *csstree.Block) *csstree.List {
	return csstree.NewListOf(&csstree.Rule{
		Prelude: &csstree.SelectorList{
			Children: csstree.NewListOf(&csstree.Selector{
				Children: csstree.NewListOf(&csstree.TypeSelector{Name: "x"}),
			}),
		},
		Block: block,
	})
}

// readChunk moves the nodes of input up to the next exclamation comment to
// keep (or to the end) into a new stylesheet, dropping other comments. It
// returns the comment that starts the chunk, if any.
func readChunk(input *csstree.List, specialComments bool) (*csstree.Comment, *csstree.StyleSheet) {
	children := csstree.NewList()
	nonSpaceTokenInBuffer := false
	var protectedComment *csstree.Comment

	input.NextUntil(input.Head, func(node csstree.Node, item *csstree.ListItem) bool {
		if comment, ok := node.(*csstree.Comment); ok {
			if !specialComments || !strings.HasPrefix(comment.Value, "!") {
				input.Remove(item)
				return false
			}
			if nonSpaceTokenInBuffer || protectedComment != nil {
				return true
			}
			input.Remove(item)
			protectedComment = comment
			return false
		}

		if _, ok := node.(*csstree.WhiteSpace); !ok {
			nonSpaceTokenInBuffer = true
		}
		children.Append(input.Remove(item))
		return false
	})

	return protectedComment, &csstree.StyleSheet{Children: children}
}

// compressChunk runs csso's passes over one chunk: clean, replace and, when
// enabled, restructure.
func (c *compressor) compressChunk(ast *csstree.StyleSheet, firstAtrulesAllowed bool, num int) error {
	c.logger("Compress block #"+strconv.Itoa(num), nil)

	c.numberBlocks(ast, firstAtrulesAllowed)
	c.logger("init", ast)

	// remove redundant
	if err := clean.Run(ast, clean.Options{Usage: c.usage, Detached: c.detached}); err != nil {
		return err
	}
	c.logger("clean", ast)

	// replace nodes for shortened forms
	if err := replace.Run(ast); err != nil {
		return err
	}
	c.logger("replace", ast)

	// structure optimisations
	if c.restructure {
		return restructure.Run(ast, restructure.Options{
			Usage:           c.usage,
			ForceMediaMerge: c.forceMediaMerge,
			Logger:          c.logger,
		})
	}
	return nil
}

// numberBlocks gives the chunk and the blocks of its at-rules the ids that
// restructuring groups rules by.
func (c *compressor) numberBlocks(ast *csstree.StyleSheet, firstAtrulesAllowed bool) {
	seed := 1
	ast.FirstAtrulesAllowed = firstAtrulesAllowed
	ast.ID = seed
	seed++

	csstree.Walk(ast, csstree.WalkOptions{
		Visit: "Atrule",
		Enter: func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem, _ *csstree.List) csstree.WalkAction {
			if block := node.(*csstree.Atrule).Block; block != nil {
				block.ID = seed
				seed++
			}
			return csstree.WalkContinue
		},
	})
}

// compressChildless handles a root without children, such as a Rule or a
// Declaration. In csso the dummy rule wrapping it counts as empty, so the
// root is only cleaned before the cleaner drops the dummy rule; replace and
// restructure then find nothing to do.
func (c *compressor) compressChildless(root csstree.Node) error {
	chunk := &csstree.StyleSheet{Children: csstree.NewList(), FirstAtrulesAllowed: true, ID: 1}
	c.logger("Compress block #1", nil)
	c.logger("init", chunk)
	if err := clean.RunNode(root, chunk, clean.Options{Usage: c.usage}); err != nil {
		return convertError(err)
	}
	c.logger("clean", chunk)
	c.logger("replace", chunk)
	if c.restructure {
		return restructure.Run(chunk, restructure.Options{Logger: c.logger})
	}
	return nil
}
