package cova

type AstProgramNode struct {
	Decls     []*AstTopLevelDeclNode
	Functions []*AstFunctionNode
}

type AstParameter struct {
	Type *Type
	Name string
	Line int
}

type AstTopLevelDeclNode struct {
	Index       int
	Name        string
	Type        *Type
	Params      []AstParameter
	Initializer AstExprNode
	Kind        DeclKind
	Scope       ScopeKind
	Line        int
}

type AstFunctionNode struct {
	ReturnType *Type
	Name       string
	Params     []AstParameter
	Body       *AstBlockStmt
	Line       int
}

type AstStmtNode interface {
	astStmtNode()
}

type AstExprNode interface {
	astExprNode()
}

type AstLvalueNode interface {
	AstExprNode
	astEmitAddress(compiler *functionCompiler)
}

type AstBlockStmt struct {
	Statements []AstStmtNode
	Line       int
}

type AstIfStmt struct {
	Condition AstExprNode
	Then      AstStmtNode
	Else      AstStmtNode
	Line      int
}

type AstWhileStmt struct {
	Condition AstExprNode
	Body      AstStmtNode
	Line      int
}

type AstForStmt struct {
	Init      AstStmtNode
	Condition AstExprNode
	Post      AstStmtNode
	Body      AstStmtNode
	Line      int
}

type AstSwitchCase struct {
	Value AstExprNode
	Body  []AstStmtNode
	Line  int
}

type AstSwitchStmt struct {
	Value   AstExprNode
	Cases   []AstSwitchCase
	Default []AstStmtNode
	Line    int
}

type AstBreakStmt struct {
	Line int
}

type AstContinueStmt struct {
	Line int
}

type AstReturnStmt struct {
	Value AstExprNode
	Line  int
}

type AstExprStmt struct {
	Expr AstExprNode
	Line int
}

type AstAssignStmt struct {
	Target AstLvalueNode
	Value  AstExprNode
	Line   int
}

type AstLocalDeclStmt struct {
	Type        *Type
	Name        string
	Initializer AstExprNode
	Line        int
}

type AstNumberLiteral struct {
	IntValue   int
	FloatValue float64
	IsFloat    bool
	FloatType  *Type
	Line       int
}

type AstStringLiteral struct {
	Value string
	Line  int
}

type AstIdentNode struct {
	Name string
	Line int
}

type AstBinaryExpr struct {
	Op    string
	Left  AstExprNode
	Right AstExprNode
	Line  int
}

type AstCallExpr struct {
	Callee string
	Args   []AstExprNode
	Line   int
}

func (*AstBlockStmt) astStmtNode()     {}
func (*AstIfStmt) astStmtNode()        {}
func (*AstWhileStmt) astStmtNode()     {}
func (*AstForStmt) astStmtNode()       {}
func (*AstSwitchStmt) astStmtNode()    {}
func (*AstReturnStmt) astStmtNode()    {}
func (*AstExprStmt) astStmtNode()      {}
func (*AstAssignStmt) astStmtNode()    {}
func (*AstLocalDeclStmt) astStmtNode() {}
func (*AstBreakStmt) astStmtNode()     {}
func (*AstContinueStmt) astStmtNode()  {}
func (*AstNumberLiteral) astExprNode() {}
func (*AstStringLiteral) astExprNode() {}
func (*AstIdentNode) astExprNode()     {}
func (*AstBinaryExpr) astExprNode()    {}
func (*AstCallExpr) astExprNode()      {}
