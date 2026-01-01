package java

import (
	"errors"
	"fmt"

	"github.com/heshanpadmasiri/javaGo/diagnostics"
	"github.com/heshanpadmasiri/javaGo/gosrc"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func convertStatementBlock(ctx *MigrationContext, blockNode *tree_sitter.Node) []gosrc.Statement {
	var body []gosrc.Statement
	IterateChilden(blockNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		// ignored
		case "{":
		case "}":
		case "line_comment":
		case "block_comment":
		default:
			body = append(body, convertStatement(ctx, child)...)
		}
	})
	return body
}

func convertSwitchStatement(ctx *MigrationContext, switchNode *tree_sitter.Node) gosrc.SwitchStatement {
	condition, conditionInit := convertExpression(ctx, switchNode.ChildByFieldName("condition"))
	Assert("condition expression is expected to be simple", len(conditionInit) == 0)
	bodyNode := switchNode.ChildByFieldName("body")
	var cases []gosrc.SwitchCase
	var defaultBody []gosrc.Statement
	IterateChilden(bodyNode, func(switchBlockStatementGroup *tree_sitter.Node) {
		switch switchBlockStatementGroup.Kind() {
		case "switch_block_statement_group":
			var caseBody []gosrc.Statement
			var caseCondition gosrc.Expression
			var isDefault bool
			IterateChilden(switchBlockStatementGroup, func(child *tree_sitter.Node) {
				switch child.Kind() {
				case "switch_label":
					if child.Utf8Text(ctx.JavaSource) == "default" {
						isDefault = true
					} else {
						caseCondition, conditionInit = convertExpression(ctx, child.Child(1))
						Assert("condition expression is expected to be simple", len(conditionInit) == 0)
					}
				// ignored
				case ":":
				case "{":
				case "}":
				case "->":
				case "line_comment":
				case "block_comment":
				default:
					var stmts []gosrc.Statement
					if child.Kind() == "block" {
						stmts = convertStatementBlock(ctx, child)
					} else {
						stmts = convertStatement(ctx, child)
					}
					if isDefault {
						defaultBody = append(defaultBody, stmts...)
					} else {
						caseBody = append(caseBody, stmts...)
					}
				}
			})
			if !isDefault {
				cases = append(cases, gosrc.SwitchCase{
					Condition: caseCondition,
					Body:      caseBody,
				})
			}
		case "switch_rule":
			caseConditionNode := switchBlockStatementGroup.Child(0)
			caseCondition := gosrc.GoExpression{Source: caseConditionNode.Utf8Text(ctx.JavaSource)}
			bodyNode := switchBlockStatementGroup.Child(2)
			for bodyNode.Kind() == "line_comment" || bodyNode.Kind() == ":" || bodyNode.Kind() == "->" {
				bodyNode = bodyNode.NextSibling()
			}
			var caseBody []gosrc.Statement
			if bodyNode.Kind() == "block" {
				caseBody = convertStatementBlock(ctx, bodyNode)
			} else {
				caseBody = convertStatement(ctx, bodyNode)
			}
			cases = append(cases, gosrc.SwitchCase{
				Condition: &caseCondition,
				Body:      caseBody,
			})
			// ignored
		case "{":
		case "}":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, switchBlockStatementGroup, "switch_block_statement_group")
		}
	})
	// TODO: if in return properly detect value points and add returns
	return gosrc.SwitchStatement{
		Condition:   condition,
		Cases:       cases,
		DefaultBody: defaultBody,
	}
}

func convertThrowStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node) []gosrc.Statement {
	valueNode := stmtNode.Child(1)
	exception := valueNode.ChildByFieldName("type").Utf8Text(ctx.JavaSource)
	arguments := valueNode.ChildByFieldName("arguments").Utf8Text(ctx.JavaSource)
	switch exception {
	case "IllegalArgumentException":
		return []gosrc.Statement{
			&gosrc.GoStatement{
				Source: fmt.Sprintf("panic(%s)", arguments),
			},
		}
	default:
		return []gosrc.Statement{
			&gosrc.GoStatement{
				Source: stmtNode.Utf8Text(ctx.JavaSource),
			},
		}
	}
}

func convertEnhancedForStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node) []gosrc.Statement {
	varName := stmtNode.ChildByFieldName("name").Utf8Text(ctx.JavaSource)
	valueExpr, stmts := convertExpression(ctx, stmtNode.ChildByFieldName("value"))
	bodyStmts := convertStatementBlock(ctx, stmtNode.ChildByFieldName("body"))
	return append(stmts, &gosrc.RangeForStatement{
		ValueVar:       varName,
		CollectionExpr: valueExpr,
		Body:           bodyStmts,
	})
}

func convertJavaForStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node) []gosrc.Statement {
	initNode := stmtNode.ChildByFieldName("init")
	var initStmts []gosrc.Statement
	if initNode != nil {
		initStmts = convertStatement(ctx, initNode)
	}
	conditionNode := stmtNode.ChildByFieldName("condition")
	conditionExp, s := convertExpression(ctx, conditionNode)
	initStmts = append(initStmts, s...)
	updateNode := stmtNode.ChildByFieldName("update")
	var updateExp gosrc.Expression
	if updateNode != nil {
		var updateStmts []gosrc.Statement
		updateExp, updateStmts = convertExpression(ctx, updateNode)
		initStmts = append(initStmts, updateStmts...)
	}
	bodyNode := stmtNode.ChildByFieldName("body")
	bodyStmts := convertStatementBlock(ctx, bodyNode)
	return append(initStmts, &gosrc.ForStatement{
		Condition: conditionExp,
		Post:      updateExp,
		Body:      bodyStmts,
	})
}

func convertWhileStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node) []gosrc.Statement {
	conditionNode := stmtNode.ChildByFieldName("condition")
	conditionExp, initStmts := convertExpression(ctx, conditionNode)
	bodyNode := stmtNode.ChildByFieldName("body")
	bodyStmts := convertStatementBlock(ctx, bodyNode)
	return append(initStmts, &gosrc.ForStatement{
		Condition: conditionExp,
		Body:      bodyStmts,
	})
}

func convertLocalVariableDeclaration(ctx *MigrationContext, stmtNode *tree_sitter.Node) []gosrc.Statement {
	typeNode := stmtNode.ChildByFieldName("type")
	ty, ok := TryParseType(ctx, typeNode)
	if !ok {
		diagnostics.Fatal(typeNode.ToSexp(), errors.New("unable to parse type in local_variable_declaration"))
	}
	declNode := stmtNode.ChildByFieldName("declarator")
	name := declNode.ChildByFieldName("name").Utf8Text(ctx.JavaSource)
	valueNode := declNode.ChildByFieldName("value")
	if valueNode == nil {
		return []gosrc.Statement{
			&gosrc.VarDeclaration{
				Name: name,
				Ty:   ty,
			},
		}
	}
	valueExpr, initStmts := convertExpression(ctx, valueNode)
	return append(initStmts, &gosrc.VarDeclaration{
		Name:  name,
		Ty:    ty,
		Value: valueExpr,
	})
}

func convertReturnStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node) []gosrc.Statement {
	var initialStmts []gosrc.Statement
	var value gosrc.Expression
	ctx.InReturn = true
	IterateChilden(stmtNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case ";":
		case "return":
		default:
			value, initialStmts = convertExpression(ctx, child)
		}
	})
	ctx.InReturn = true
	// Check if value is a gosrc.SwitchStatement
	if switchStmt, ok := value.(*gosrc.SwitchStatement); ok {
		// If value is a gosrc.SwitchStatement, flatten to its switch form
		// Not conventional return, treat as statement
		return append(initialStmts, switchStmt)
	}
	return append(initialStmts, &gosrc.ReturnStatement{Value: value})
}

func convertExpressionStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node) []gosrc.Statement {
	var body []gosrc.Statement
	IterateChilden(stmtNode, func(child *tree_sitter.Node) {
		switch child.Kind() {
		case "assignment_expression":
			_, stmts := convertAssignmentExpression(ctx, child)
			body = append(body, stmts...)
		case "method_invocation":
			// Check if this is a .add() call that should be converted to append
			methodName := child.ChildByFieldName("name").Utf8Text(ctx.JavaSource)
			objectNode := child.ChildByFieldName("object")

			if methodName == "add" && objectNode != nil {
				// Convert list.add(item) to list = append(list, item)
				objectText := objectNode.Utf8Text(ctx.JavaSource)
				argsNode := child.ChildByFieldName("arguments")
				if argsNode != nil {
					args := convertArgumentList(ctx, argsNode)
					if len(args) > 0 {
						// Create: list = append(list, item)
						body = append(body, &gosrc.AssignStatement{
							Ref: gosrc.VarRef{Ref: objectText},
							Value: &gosrc.CallExpression{
								Function: "append",
								Args:     append([]gosrc.Expression{&gosrc.VarRef{Ref: objectText}}, args...),
							},
						})
					} else {
						// Fall through to regular method call handling
						callExperession, initStmts := convertExpression(ctx, child)
						body = append(body, initStmts...)
						body = append(body, &gosrc.CallStatement{Exp: callExperession})
					}
				} else {
					// Fall through to regular method call handling
					callExperession, initStmts := convertExpression(ctx, child)
					body = append(body, initStmts...)
					body = append(body, &gosrc.CallStatement{Exp: callExperession})
				}
			} else {
				callExperession, initStmts := convertExpression(ctx, child)
				body = append(body, initStmts...)
				body = append(body, &gosrc.CallStatement{Exp: callExperession})
			}
		// ignored
		case ";":
		default:
			expr, initStmts := convertExpression(ctx, child)
			body = append(body, initStmts...)
			body = append(body, &gosrc.GoStatement{Source: expr.ToSource() + ";"})
		}
	})
	return body
}

func convertStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node) []gosrc.Statement {
	switch stmtNode.Kind() {
	case "line_comment":
		return nil
	case "block_comment":
		return nil
	case "switch_expression":
		switchStatement := convertSwitchStatement(ctx, stmtNode)
		return []gosrc.Statement{&switchStatement}
	case "assert_statement":
		conditionNode := stmtNode.Child(1)
		conditionExp, initStmts := convertExpression(ctx, conditionNode)
		Assert("condition expression is expected to be simple", len(initStmts) == 0)
		return append(initStmts, &gosrc.IfStatement{
			Condition: conditionExp,
			Body:      []gosrc.Statement{&gosrc.GoStatement{Source: "panic(\"assertion failed\")"}},
		})
	case "expression_statement":
		return convertExpressionStatement(ctx, stmtNode)
	case "return_statement":
		return convertReturnStatement(ctx, stmtNode)
	case "if_statement":
		ifStatement := convertIfStatement(ctx, stmtNode, false)
		return []gosrc.Statement{&ifStatement}
	case "break_statement":
		return []gosrc.Statement{&gosrc.GoStatement{Source: "break;"}}
	case "continue_statement":
		return []gosrc.Statement{&gosrc.GoStatement{Source: "continue;"}}
	case "local_variable_declaration":
		return convertLocalVariableDeclaration(ctx, stmtNode)
	case "while_statement":
		return convertWhileStatement(ctx, stmtNode)
	case "for_statement":
		return convertJavaForStatement(ctx, stmtNode)
	case "enhanced_for_statement":
		return convertEnhancedForStatement(ctx, stmtNode)
	case "throw_statement":
		return convertThrowStatement(ctx, stmtNode)
	case ";":
		return nil
	case "yield_statement":
		expr, init := convertExpression(ctx, stmtNode.Child(1))
		init = append(init, &gosrc.GoStatement{Source: expr.ToSource() + ";"})
		return init
	case "try_statement":
		tryStatement := convertTryStatement(ctx, stmtNode)
		return []gosrc.Statement{&tryStatement}
	default:
		expr, init := convertExpression(ctx, stmtNode)
		init = append(init, &gosrc.GoStatement{Source: expr.ToSource() + ";"})
		return init
	}
}

func convertTryStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node) gosrc.TryStatement {
	var tryBody []gosrc.Statement
	var catchClauses []gosrc.CatchClause
	var finallyBody []gosrc.Statement

	// Get try body
	bodyNode := stmtNode.ChildByFieldName("body")
	if bodyNode != nil {
		tryBody = convertStatementBlock(ctx, bodyNode)
	}

	// Check for finally using field name
	finallyNode := stmtNode.ChildByFieldName("finally")
	if finallyNode != nil {
		finallyBodyNode := finallyNode.ChildByFieldName("body")
		if finallyBodyNode != nil {
			finallyBody = convertStatementBlock(ctx, finallyBodyNode)
		} else if finallyNode.Kind() == "block" {
			finallyBody = convertStatementBlock(ctx, finallyNode)
		}
	}

	// Iterate through children to find catch clauses and finally
	IterateChilden(stmtNode, func(child *tree_sitter.Node) {
		if child.Kind() == "catch_clause" {
			var exceptionType string
			var exceptionVar string
			var catchBody []gosrc.Statement

			// Find catch_formal_parameter
			IterateChilden(child, func(catchChild *tree_sitter.Node) {
				if catchChild.Kind() == "catch_formal_parameter" {
					// Find catch_type
					IterateChilden(catchChild, func(paramChild *tree_sitter.Node) {
						if paramChild.Kind() == "catch_type" {
							// Get the type identifier from catch_type
							IterateChilden(paramChild, func(typeChild *tree_sitter.Node) {
								if typeChild.Kind() == "type_identifier" || typeChild.Kind() == "scoped_type_identifier" {
									exceptionType = typeChild.Utf8Text(ctx.JavaSource)
								}
							})
						}
					})
					// Get name field
					nameNode := catchChild.ChildByFieldName("name")
					if nameNode != nil {
						exceptionVar = nameNode.Utf8Text(ctx.JavaSource)
					}
				}
			})
			// Get catch body
			catchBodyNode := child.ChildByFieldName("body")
			if catchBodyNode != nil {
				catchBody = convertStatementBlock(ctx, catchBodyNode)
			}

			if exceptionType != "" {
				catchClauses = append(catchClauses, gosrc.CatchClause{
					ExceptionType: exceptionType,
					ExceptionVar:  exceptionVar,
					Body:          catchBody,
				})
			}
		} else if child.Kind() == "finally_clause" {
			// Get finally body
			finallyBodyNode := child.ChildByFieldName("body")
			if finallyBodyNode != nil {
				finallyBody = convertStatementBlock(ctx, finallyBodyNode)
			} else {
				// Look for block as direct child
				IterateChilden(child, func(fc *tree_sitter.Node) {
					if fc.Kind() == "block" {
						finallyBody = convertStatementBlock(ctx, fc)
					}
				})
			}
		}
	})

	return gosrc.TryStatement{
		TryBody:      tryBody,
		CatchClauses: catchClauses,
		FinallyBody:  finallyBody,
	}
}

func convertIfStatement(ctx *MigrationContext, stmtNode *tree_sitter.Node, inner bool) gosrc.IfStatement {
	conditionNode := stmtNode.ChildByFieldName("condition")
	conditionExp, stmts := convertExpression(ctx, conditionNode)
	Assert("condition expression is expected to be simple", len(stmts) == 0)
	bodyNode := stmtNode.ChildByFieldName("consequence")
	bodyStmts := convertStatementBlock(ctx, bodyNode)
	ifStatement := &gosrc.IfStatement{
		Condition: conditionExp,
		Body:      bodyStmts,
	}
	cursor := stmtNode.Walk()
	elseIf := stmtNode.ChildrenByFieldName("alternative", cursor)
	for _, elseIfNode := range elseIf {
		switch elseIfNode.Kind() {
		case "if_statement":
			ifStatement.ElseIf = append(ifStatement.ElseIf, convertIfStatement(ctx, &elseIfNode, true))
		case "block":
			elseBodyStmts := convertStatementBlock(ctx, &elseIfNode)
			ifStatement.ElseStmts = append(ifStatement.ElseStmts, elseBodyStmts...)
		default:
			UnhandledChild(ctx, &elseIfNode, "else_if_statement")
		}
	}
	return *ifStatement
}

// Check for finally using field name
func convertExplicitConstructorInvocation(ctx *MigrationContext, invocationNode *tree_sitter.Node) []gosrc.Statement {
	parentCall := "this"
	var argExp []gosrc.Expression
	IterateChilden(invocationNode, func(args *tree_sitter.Node) {
		switch args.Kind() {
		case "this":
			parentCall = "this"
		case "super":
			parentCall = "super"
		case "argument_list":
			argExp = convertArgumentList(ctx, args)
		// ignored
		case ";":
		case "line_comment":
		case "block_comment":
		default:
			UnhandledChild(ctx, args, "explicit_constructor_invocation")
		}
	})
	return []gosrc.Statement{
		&gosrc.CallStatement{
			Exp: &gosrc.CallExpression{
				Function: parentCall,
				Args:     argExp,
			},
		},
	}
}
