package lcc

type ProgramNode struct {
	Decls     []*TopLevelDeclNode
	Functions []*FunctionNode
}

type Parameter struct {
	Type *Type
	Name string
	Line int
}

type TopLevelDeclNode struct {
	Index       int
	Name        string
	Type        *Type
	Params      []Parameter
	Initializer ExprNode
	Kind        DeclKind
	Scope       ScopeKind
	Line        int
}

type FunctionNode struct {
	ReturnType *Type
	Name       string
	Params     []Parameter
	Body       *BlockStmt
	Line       int
}

type StmtNode interface {
	stmtNode()
}

type ExprNode interface {
	exprNode()
}

type LvalueNode interface {
	ExprNode
	EmitAddress(code *CodeMemory, c *Compiler)
}

type BlockStmt struct {
	Statements []StmtNode
	Line       int
}

type IfStmt struct {
	Condition ExprNode
	Then      StmtNode
	Else      StmtNode
	Line      int
}

type WhileStmt struct {
	Condition ExprNode
	Body      StmtNode
	Line      int
}

type ForStmt struct {
	Init      StmtNode
	Condition ExprNode
	Post      StmtNode
	Body      StmtNode
	Line      int
}

type SwitchCase struct {
	Value ExprNode
	Body  []StmtNode
	Line  int
}

type SwitchStmt struct {
	Value   ExprNode
	Cases   []SwitchCase
	Default []StmtNode
	Line    int
}

type BreakStmt struct {
	Line int
}

type ContinueStmt struct {
	Line int
}

type ReturnStmt struct {
	Value ExprNode
	Line  int
}

type ExprStmt struct {
	Expr ExprNode
	Line int
}

type AssignStmt struct {
	Target LvalueNode
	Value  ExprNode
	Line   int
}

type LocalDeclStmt struct {
	Type        *Type
	Name        string
	Initializer ExprNode
	Line        int
}

type NumberLiteral struct {
	IntValue   int
	FloatValue float64
	IsFloat    bool
	FloatType  *Type
	Line       int
}

type StringLiteral struct {
	Value string
	Line  int
}

type IdentNode struct {
	Name string
	Line int
}

type BinaryExpr struct {
	Op    string
	Left  ExprNode
	Right ExprNode
	Line  int
}

type CallExpr struct {
	Callee string
	Args   []ExprNode
	Line   int
}

func (*BlockStmt) stmtNode()     {}
func (*IfStmt) stmtNode()        {}
func (*WhileStmt) stmtNode()     {}
func (*ForStmt) stmtNode()       {}
func (*SwitchStmt) stmtNode()    {}
func (*ReturnStmt) stmtNode()    {}
func (*ExprStmt) stmtNode()      {}
func (*AssignStmt) stmtNode()    {}
func (*LocalDeclStmt) stmtNode() {}
func (*BreakStmt) stmtNode()     {}
func (*ContinueStmt) stmtNode()  {}
func (*NumberLiteral) exprNode() {}
func (*StringLiteral) exprNode() {}
func (*IdentNode) exprNode()     {}
func (*BinaryExpr) exprNode()    {}
func (*CallExpr) exprNode()      {}