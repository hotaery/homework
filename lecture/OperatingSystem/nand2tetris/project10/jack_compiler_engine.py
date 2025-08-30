from jack_tokenizer import JackTokenizer
from jack_token import Token, TokenType, Keyword
from xml.etree import ElementTree as ET
from typing import Optional, Tuple, Iterable, Dict, Callable

class JackCompilerEngine(object):
    statements_route: Dict[str, str] = {
        Keyword.LET:        "_compileLet",
        Keyword.IF:         "_compileIf",
        Keyword.WHILE:      "_compileWhile",
        Keyword.DO:         "_compileDo",
        Keyword.RETURN:     "_compileReturn"
    }

    def __init__(self, token_iter: Iterable):
        self._token_iter = token_iter

    def parse(self) -> ET.Element:
        return self._compileClass(next(self._token_iter))

    def _compileClass(self, token: Token) -> ET.Element:
        assert token.token_type == TokenType.KEYWORD and token.value == Keyword.CLASS
        root = ET.Element("class")
        while True:
            token_type, token_value = JackTokenizer.tokenToString(token)
            elem = ET.Element(token_type)
            elem.text = token_value
            if token_value == 'static' or token_value == 'field':
                class_var_dec = self._compileClassVarDec(token)
                root.append(class_var_dec)
            elif token.value in [Keyword.CONSTRUCTOR, Keyword.FUNCTION, Keyword.METHOD]:
                subroutine_dec = self._compileSubroutineDec(token)
                root.append(subroutine_dec)
            else:
                root.append(elem)
                if token_value == '}':
                    break
            token = next(self._token_iter)
        return root

    def _compileClassVarDec(self, token: Token) -> ET.Element:
        assert token.token_type == TokenType.KEYWORD and \
            (token.value == Keyword.STATIC or token.value == Keyword.FIELD)
        root = ET.Element("classVarDec")
        while True:
            token_type, token_value = JackTokenizer.tokenToString(token)
            elem = ET.Element(token_type)
            elem.text = token_value
            root.append(elem)
            if token_value == ';':
                break
            token = next(self._token_iter)
        return root

    def _compileSubroutineDec(self, token: Token) -> ET.Element:
        assert token.token_type == TokenType.KEYWORD and \
            (token.value == Keyword.CONSTRUCTOR or token.value == Keyword.METHOD or 
             token.value == Keyword.FUNCTION)
        root = ET.Element("subroutineDec")
        while True:
            token_type, token_value = JackTokenizer.tokenToString(token)
            elem = ET.Element(token_type)
            elem.text = token_value
            if token_value == '(':
                root.append(elem)
                parameter_list, token = self._compileParameterList(next(self._token_iter))
                root.append(parameter_list)
                assert token.token_type == TokenType.SYMBOL and \
                    token.value == ')'
            elif token_value == '{':
                body = self._compileSubroutineBody(token)
                root.append(body)
                break
            else:
                root.append(elem)
                token = next(self._token_iter)
        return root

    def _compileParameterList(self, token: Token) -> Tuple[ET.Element, Token]:
        root = ET.Element("parameterList")
        root.text = "\n"
        while True:
            if token.token_type == TokenType.SYMBOL and token.value == ')':
                break
            token_type, token_value = JackTokenizer.tokenToString(token)
            elem = ET.Element(token_type)
            elem.text = token_value
            root.append(elem)
            token = next(self._token_iter)
        return root, token

    def _compileSubroutineBody(self, token: Token) -> ET.Element:
        assert token.token_type == TokenType.SYMBOL and \
            token.value == '{'
        root = ET.Element("subroutineBody")
        token_type, token_value = JackTokenizer.tokenToString(token)
        elem = ET.Element(token_type)
        elem.text = token_value
        root.append(elem)
        token = next(self._token_iter)
        while True:
            token_type, token_value = JackTokenizer.tokenToString(token)
            if token.token_type == TokenType.KEYWORD and token.value == Keyword.VAR:
                var_dec = self._compileVarDec(token)
                root.append(var_dec)
                token = next(self._token_iter)
            elif token.token_type == TokenType.KEYWORD:
                statements, token = self._compileStatements(token)
                root.append(statements)
                assert token.token_type == TokenType.SYMBOL and token.value == '}'
            else:
                assert token_value == '}'
                elem = ET.Element(token_type)
                elem.text = token_value
                root.append(elem)
                break
        return root

    def _compileVarDec(self, token: Token) -> ET.Element:
        assert token.token_type == TokenType.KEYWORD and token.value == Keyword.VAR
        root = ET.Element("varDec")
        while True:
            token_type, token_value = JackTokenizer.tokenToString(token)
            elem = ET.Element(token_type)
            elem.text = token_value
            root.append(elem)
            if token_value == ';':
                break
            token = next(self._token_iter)
        return root

    def _compileStatements(self, token: Token) -> Tuple[ET.Element, Token]:
        root = ET.Element("statements")
        if not (token.token_type == TokenType.KEYWORD and \
            token.value in self.statements_route):
            root.text = "\n"
            return root, token
        while True:
            if token.token_type != TokenType.KEYWORD:
                break
            method = self.statements_route.get(token.value)
            if not method:
                break
            method = getattr(self, method)
            statement = method(token)
            token = None
            if isinstance(statement, tuple):
                # _compileIf
                token = statement[1]
                statement = statement[0]
            root.append(statement)
            if token == None:
                token = next(self._token_iter)
        return root, token

    def _compileLet(self, token: Token) -> ET.Element:
        assert token.token_type == TokenType.KEYWORD and \
            token.value == Keyword.LET
        root = ET.Element("letStatement")
        end = False
        while not end:
            token_type, token_value = JackTokenizer.tokenToString(token)
            elem = ET.Element(token_type)
            elem.text = token_value
            root.append(elem)
            if token_value == '[':
                # array
                exp, token = self._compileExpression(next(self._token_iter))
                root.append(exp)
                if token == None:
                    token = next(self._token_iter)
                assert token.token_type == TokenType.SYMBOL and token.value == ']'
            elif token_value == '=':
                exp, token = self._compileExpression(next(self._token_iter))
                root.append(exp)
                if token == None:
                    token = next(self._token_iter)
                assert token.token_type == TokenType.SYMBOL and token.value == ';'
            elif token_value == ';':
                end = True
            else:
                token = next(self._token_iter) 
        return root

    def _compileIf(self, token: Token) -> Tuple[ET.Element, Token]:
        # 'if' '(' expression ')' '{' statements '}' ( 'else' '{' statements '}' )?
        assert token.token_type == TokenType.KEYWORD and token.value == Keyword.IF
        root = ET.Element("ifStatement")
        end = False
        while not end:
            token_type, token_value = JackTokenizer.tokenToString(token)
            elem = ET.Element(token_type)
            elem.text = token_value
            root.append(elem)
            if token_value == '(':
                exp, token = self._compileExpression(next(self._token_iter))
                root.append(exp)
                if token == None:
                    token = next(self._token_iter)
                assert token.token_type == TokenType.SYMBOL and token.value == ')'
            elif token_value == '{':
                statements, token = self._compileStatements(next(self._token_iter))
                root.append(statements)
                if token == None:
                    token = next(self._token_iter)
                assert token.token_type == TokenType.SYMBOL and token.value == '}'
            elif token_value == '}':
                try:
                    token = next(self._token_iter)
                    if token.token_type != TokenType.KEYWORD or token.value != Keyword.ELSE:
                        end = True
                except:
                    token = None
                    end = True
            else:
                token = next(self._token_iter)
        return root, token

    def _compileWhile(self, token: Token) -> ET.Element:
        assert token.token_type == TokenType.KEYWORD and token.value == Keyword.WHILE
        root = ET.Element("whileStatement")
        end = False
        while not end:
            token_type, token_value = JackTokenizer.tokenToString(token)
            elem = ET.Element(token_type)
            elem.text = token_value
            root.append(elem)
            if token_value == '(':
                exp, token = self._compileExpression(next(self._token_iter))
                root.append(exp)
                if token == None:
                    token = next(self._token_iter)
                assert token.token_type == TokenType.SYMBOL and token.value == ')'
            elif token_value == '{':
                statements, token = self._compileStatements(next(self._token_iter))
                root.append(statements)
                if token == None:
                    token = next(self._token_iter)
                assert token.token_type == TokenType.SYMBOL and token.value == '}'
            elif token_value == '}':
                end = True
            else:
                token = next(self._token_iter)
        return root

    def _compileDo(self, token: Token) -> ET.Element:
        assert token.token_type == TokenType.KEYWORD and \
            token.value == Keyword.DO
        root = ET.Element("doStatement")
        token_type, token_value = JackTokenizer.tokenToString(token) 
        do = ET.Element(token_type)
        do.text = token_value
        root.append(do)
        root = self._compileSubroutineCall(next(self._token_iter), None, root)
        token = next(self._token_iter)
        assert token.token_type == TokenType.SYMBOL and \
            token.value == ';'
        token_type, token_value = JackTokenizer.tokenToString(token) 
        end = ET.Element(token_type)
        end.text = token_value
        root.append(end)
        return root 

    def _compileReturn(self, token: Token) -> ET.Element:
        assert token.token_type == TokenType.KEYWORD and \
            token.value == Keyword.RETURN
        root = ET.Element("returnStatement")
        token_type, token_value = JackTokenizer.tokenToString(token) 
        ret = ET.Element(token_type)
        ret.text = token_value
        root.append(ret)
        token = next(self._token_iter)
        if token.token_type != TokenType.SYMBOL or token.value != ';':
            exp, token = self._compileExpression(token)
            root.append(exp)
            if token == None:
                token = next(self._token_iter)
        assert token.token_type == TokenType.SYMBOL and token.value == ';'
        token_type, token_value = JackTokenizer.tokenToString(token) 
        end = ET.Element(token_type)
        end.text = token_value
        root.append(end)
        return root 

    def _compileTerm(self, token: Token, root: Optional[ET.Element]) -> Tuple[ET.Element, Optional[Token]]:
        if root == None:
            root = ET.Element("term")
        if token.token_type == TokenType.INT_CONST or \
            token.token_type == TokenType.STRING_CONST or \
            self._isKeywordConstant(token):
            # integerConstant | stringConstant | keywordConstant
            token_type, token_value = JackTokenizer.tokenToString(token)
            term = ET.Element(token_type)
            term.text = token_value
            root.append(term)
            return root, None

        if self._isUnaryOP(token):
            # unaryOp term
            unary_op = ET.Element("symbol")
            unary_op.text = token.value
            root.append(unary_op)
            term, token = self._compileTerm(next(self._token_iter), None)
            root.append(term)
            return root, token
        
        if token.value == "(":
            # '(' expression ')'
            symbol = ET.Element("symbol")
            symbol.text = "("
            root.append(symbol)
            exp, token = self._compileExpression(next(self._token_iter))
            root.append(exp)
            assert token.value == ")"
            symbol = ET.Element("symbol")
            symbol.text = ")"
            root.append(symbol)
            return root, None

        if token.token_type == TokenType.IDENTIFIER:
            # varName | varName '[' expression ']' | subroutineCall
            next_token = next(self._token_iter)
            if next_token.token_type == TokenType.SYMBOL and \
                next_token.value == '[':
                # varName '[' expression ']' 
                name = ET.Element("identifier")
                name.text = token.value
                root.append(name)  
                symbol = ET.Element("symbol")
                symbol.text = '['
                root.append(symbol)
                exp, token = self._compileExpression(next(self._token_iter))
                root.append(exp)
                assert token.value == ']'
                symbol = ET.Element("symbol")
                symbol.text = ']'
                root.append(symbol)
                return root, None
            elif next_token.token_type == TokenType.SYMBOL and \
                (next_token.value == '.' or next_token.value == '('):
                # subroutineCall
                root = self._compileSubroutineCall(token, next_token, root)
                return root, None
            else:
                # varName
                name = ET.Element("identifier")
                name.text = token.value
                root.append(name)  
                return root, next_token
        
        assert False, f"Can not reach here {token} {token.value}"

    def _compileSubroutineCall(self, token: Token, next_token: Optional[Token], root: ET.Element) -> ET.Element:
        # subroutineName '(' expressionList ')' | 
        # ( className | varName) '.' subroutineName '(' expressionList ')'
        assert token.token_type == TokenType.IDENTIFIER
        token_type, token_value = JackTokenizer.tokenToString(token)
        name = ET.Element(token_type)
        name.text = token_value
        root.append(name)
        if next_token == None:
            next_token = next(self._token_iter)
        while True:
            # '.'subroutineName
            token_type, token_value = JackTokenizer.tokenToString(next_token)
            elem = ET.Element(token_type)
            elem.text = token_value
            root.append(elem)
            if token_value == '(':
                break
            next_token = next(self._token_iter)

        exp, next_token = self._compileExpressionList(next(self._token_iter))
        assert next_token.value == ')'
        root.append(exp)
        symbol = ET.Element("symbol")
        symbol.text = ')'
        root.append(symbol)
        return root
        
    def _compileExpressionList(self, token: Token) -> Tuple[ET.Element, Token]:
        # (expression (',' expression)* )?
        root = ET.Element("expressionList")
        if token.token_type == TokenType.SYMBOL and token.value == ')':
            root.text = "\n"
            return root, token
        while True:
            exp, token = self._compileExpression(token)
            root.append(exp)
            if not (token.token_type == TokenType.SYMBOL and token.value == ','):
                break
            symbol = ET.Element("symbol")
            symbol.text = ','
            root.append(symbol)
            token = next(self._token_iter)
        return root, token

    def _compileExpression(self, token: Token) -> Tuple[ET.Element, Optional[Token]]:
        # term (op term)* 
        root = ET.Element("expression")
        lhs, token = self._compileTerm(token, None)
        root.append(lhs)
        
        while True:
            if token == None:
                token = next(self._token_iter)
            if not self._isOp(token):
                break
            symbol = ET.Element("symbol")
            symbol.text = token.value
            root.append(symbol)
            rhs, token = self._compileTerm(next(self._token_iter), None)
            root.append(rhs)

        return root, token

    def _isOp(self, token: Token) -> bool:
        return token.token_type == TokenType.SYMBOL and \
            token.value in ['+', '-', '*', '/', '&', '|', '<', '>', '=']

    def _isUnaryOP(self, token: Token) -> bool:
        return token.token_type == TokenType.SYMBOL and \
            token.value in ["-", "~"]

    def _isKeywordConstant(self, token: Token) -> bool:
        return token.token_type == TokenType.KEYWORD and \
            token.value in [Keyword.FALSE, Keyword.TRUE, Keyword.NULL, Keyword.THIS]


def _test_term():
    # integerConstant
    engine = JackCompilerEngine(None)
    token = Token(TokenType.INT_CONST, 2)
    elem, _ = engine._compileTerm(token, None)
    ET.dump(elem)

    # keywordConstant
    token = Token(TokenType.KEYWORD, Keyword.TRUE)
    elem, _ = engine._compileTerm(token, None)
    ET.dump(elem)

    # varName
    tokens = [Token(TokenType.SYMBOL, ';')]
    engine = JackCompilerEngine(iter(tokens))
    token = Token(TokenType.IDENTIFIER, "i")
    elem, token = engine._compileTerm(token, None)
    ET.dump(elem)
    assert token == tokens[0]

    # unaryOp term
    tokens = [Token(TokenType.INT_CONST, "2")]
    token = Token(TokenType.SYMBOL, '-')
    engine = JackCompilerEngine(iter(tokens))
    elem, _ = engine._compileTerm(token, None)
    ET.dump(elem)

    # '(' expression ')'
    tokens = [Token(TokenType.INT_CONST, 2), Token(TokenType.SYMBOL, '-'), 
              Token(TokenType.INT_CONST, 1), Token(TokenType.SYMBOL, ')')]
    engine = JackCompilerEngine(iter(tokens))
    token = Token(TokenType.SYMBOL, '(')
    elem, token = engine._compileTerm(token, None)
    assert token == None
    ET.dump(elem)

    # varName '[' expression ']'
    tokens = [Token(TokenType.SYMBOL, '['), Token(TokenType.INT_CONST, 2),
              Token(TokenType.SYMBOL, ']')]
    engine = JackCompilerEngine(iter(tokens))
    token = Token(TokenType.IDENTIFIER, "arr")
    elem, token = engine._compileTerm(token, None)
    assert token == None
    ET.dump(elem)

    # subroutineCall
    tokens = [Token(TokenType.SYMBOL, '('), Token(TokenType.SYMBOL, ')')]
    engine = JackCompilerEngine(iter(tokens))
    token = Token(TokenType.IDENTIFIER, "fn")
    elem, token = engine._compileTerm(token, None)
    assert token == None
    ET.dump(elem)

    # subroutineCall
    # foo.set(a + (b * 43), a * foo.get())
    tokens = [Token(TokenType.SYMBOL, '.'),     Token(TokenType.IDENTIFIER, "set"),
              Token(TokenType.SYMBOL, '('),     Token(TokenType.IDENTIFIER, 'a'),
              Token(TokenType.SYMBOL, '+'),     Token(TokenType.SYMBOL, '('),
              Token(TokenType.IDENTIFIER, 'b'), Token(TokenType.SYMBOL, '*'),
              Token(TokenType.INT_CONST, 43),   Token(TokenType.SYMBOL, ')'),
              Token(TokenType.SYMBOL, ','),     Token(TokenType.IDENTIFIER, 'a'), 
              Token(TokenType.SYMBOL, '*'),     Token(TokenType.IDENTIFIER, "foo"),
              Token(TokenType.SYMBOL, '.'),     Token(TokenType.IDENTIFIER, "get"),
              Token(TokenType.SYMBOL, '('),     Token(TokenType.SYMBOL, ')'),     
              Token(TokenType.SYMBOL, ')')]
    engine = JackCompilerEngine(iter(tokens))
    token = Token(TokenType.IDENTIFIER, "foo")
    elem, token = engine._compileTerm(token, None)
    assert not token
    ET.dump(elem)

def _test_let():
    token = Token(TokenType.KEYWORD, Keyword.LET)
    # let a=1;
    tokens = [Token(TokenType.IDENTIFIER, 'a'), Token(TokenType.SYMBOL, '='),
              Token(TokenType.INT_CONST, '1'),  Token(TokenType.SYMBOL, ';')]
    engine = JackCompilerEngine(iter(tokens))
    elem = engine._compileLet(token)
    ET.dump(elem)

    # let arr[Math.add(a, b)] = 5;
    tokens = [Token(TokenType.IDENTIFIER, 'arr'),   Token(TokenType.SYMBOL, '['),
              Token(TokenType.IDENTIFIER, 'Math'),  Token(TokenType.SYMBOL, '.'),
              Token(TokenType.IDENTIFIER, 'add'),   Token(TokenType.SYMBOL, '('),
              Token(TokenType.IDENTIFIER, 'a'),     Token(TokenType.SYMBOL, ','),
              Token(TokenType.IDENTIFIER, 'b'),     Token(TokenType.SYMBOL, ')'),
              Token(TokenType.SYMBOL, ']'),         Token(TokenType.SYMBOL, '='),
              Token(TokenType.INT_CONST, 5),        Token(TokenType.SYMBOL, ';')]
    engine = JackCompilerEngine(iter(tokens))
    elem = engine._compileLet(token)
    ET.dump(elem)

def _test_if():
    import tempfile
    import os
    content = """if (foo.flag()) {
    let a = Math.add(foo.getA(), foo.getB());
    do foo.cancel();
    return a;
    } else {
        while ((foo.state() < 0)|(foo.state() = 0)) {
            do foo.setA(foo.getA() + 1);
            do foo.setB(foo.getB() + 1);
        }
    }
    """
    file = tempfile.mktemp()
    with open(file, "w+") as f:
        f.write(content)
    tokenizer = JackTokenizer(file)
    iter_ = iter(tokenizer)
    token = next(iter_)
    engine = JackCompilerEngine(iter_)
    elem, token = engine._compileIf(token)
    assert not token
    ET.dump(elem)
    os.unlink(file)

def _test_class():
    import tempfile
    import os
    content = """
class Student {
    static int global_id;
    field int _id, _age;
    field Array _name;
    field School _school;

    constructor Student new(int age, Array name, School school) {
        let _id = global_id;
        let global_id = global_id + 1;
        let _age = age;
        let _name = Format.formatName(name);
        let _school = school;
        return this;
    }

    function int getGlobalId() {
        return global_id;
    }

    method int id() {
        return _id;
    }

    method int age() {
        return _age;
    } 

    method Array name() {
        return _name;
    }

    method School school() {
        return _school;
    }

    method void setAge(int age) {
        let _age = age;
    }

    method void setName(Array name) {
        let _name = Format.formatName(name);
    }

    method void setSchool(School school) {
        let _school = school;
    }
}
"""
    file = tempfile.mktemp()
    with open(file, "w+") as f:
        f.write(content)
    
    tokenizer = JackTokenizer(file)
    token_iter = iter(tokenizer)
    token = next(tokenizer)
    engine = JackCompilerEngine(token_iter)
    try:
        elem = engine._compileClass(token)
        ET.dump(elem)
    except:
        os.unlink(file)
        raise

def _test():
    import sys
    tokenizer = JackTokenizer(sys.argv[2])
    engine = JackCompilerEngine(iter(tokenizer))
    elem = engine.parse()
    ET.dump(elem)


if __name__ == "__main__":
    import sys
    test_map: Dict[str, Callable[[], None]] = {
        "term":     _test_term,
        "let":      _test_let,
        "if":       _test_if,
        "class":    _test_class,
        "test":     _test,
    }

    test_fn = test_map.get(sys.argv[1])
    test_fn()
