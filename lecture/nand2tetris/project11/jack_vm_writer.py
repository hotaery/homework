from jack_symbol_table import JackSymbolTable, str2Kind, SymbolKind, kind2Str
from xml.etree import ElementTree as ET
from enum import Enum, auto
from typing import List, Tuple, Optional, Dict, Callable


class SegmentType(Enum):
    CONST       = auto()
    ARG         = auto()
    LOCAL       = auto()
    STATIC      = auto()
    THIS        = auto()
    THAT        = auto()
    POINTER     = auto()
    TEMP        = auto()


class JackVMWriter(object):
    op_map_vm_code: Dict[str, str] = {
        "+": "add",
        "-": "sub",
        "*": "call Math.multiply 2",
        "/": "call Math.divide 2",
        "&": "and",
        "|": "or",
        "<": "lt",
        ">": "gt",
        "=": "eq",
    }

    statement_route: Dict[str, str] = {
        "letStatement":     "_compileLet",
        "ifStatement":      "_compileIf",
        "whileStatement":   "_compileWhile",
        "doStatement":      "_compileDo",
        "returnStatement":  "_compileReturn",
    }

    def __init__(self, parse_tree: ET.Element, output_file: str):
        assert parse_tree.tag == "class", "Tag of parse tree must be `class'"
        if output_file:
            self._file = open(output_file, "w+")
        self._parse_tree = parse_tree
        class_identifier = self._parse_tree.find("identifier")
        assert class_identifier != None, "Identifier of class not exist"
        self._class_identifier = class_identifier.text
        self._symbol_table = JackSymbolTable(self._class_identifier)
        self._subroutine_identifier = ""
        self._label_index = 0

    def run(self):
        # class variable symbol
        self._defineClassVar(self._parse_tree.findall("classVarDec"))
        subroutine_list = self._parse_tree.findall("subroutineDec")
        for subroutine in subroutine_list:
            self._writeSubroutine(subroutine)

    def _writeSubroutine(self, subroutine: ET.Element):
        print("==============")
        assert subroutine.tag == "subroutineDec"
        identifier_elem_list = subroutine.findall("identifier")
        if len(identifier_elem_list) == 1:
            self._subroutine_identifier = identifier_elem_list[0].text
        elif len(identifier_elem_list) == 2:
            self._subroutine_identifier = identifier_elem_list[1].text
        else:
            assert False
        subrutine_type_elem = subroutine.find("keyword").text
        assert subrutine_type_elem in ["constructor", "function", "method"]
        print(f"start compile subroutine {self._class_identifier}.{self._subroutine_identifier}")
        self._label_index = 0
        self._symbol_table.startSubroutine(self._subroutine_identifier)
        self._defineSubroutineArg(subroutine.find("parameterList"))
        self._defineSubroutineVar(subroutine)
        vm_code_list = []
        n_locals = self._symbol_table.varCount(SymbolKind.VAR)
        vm_code_list.append(f"function {self._class_identifier}.{self._subroutine_identifier} {n_locals}")
        if subrutine_type_elem == "constructor":
            size = self._symbol_table.varCount(SymbolKind.FIELD)
            if size == 0:
                size = 1
            vm_code_list.append(f"    push constant {size}")
            vm_code_list.append( "    call Memory.alloc 1")
            vm_code_list.append( "    pop pointer 0")
        elif subrutine_type_elem == "method":
            vm_code_list.append( "    push argument 0")
            vm_code_list.append( "    pop pointer 0")
            self._symbol_table.inc_arg_index()
        statements = subroutine.find("subroutineBody").find("statements")
        vm_code_list = vm_code_list + self._compileStatements(statements)
        self._writeVMCode(vm_code_list)

    def _writeVMCode(self, vm_code_list: List[str]):
        vm_code_list = [code + "\n" for code in vm_code_list]
        self._file.writelines(vm_code_list)

    def _compileStatements(self, statements: ET.Element) -> List[str]:
        assert statements.tag == "statements"
        vm_code_list = []
        for child in statements:
            method = self.statement_route.get(child.tag)
            method = getattr(self, method)
            statement_code = method(child)
            vm_code_list += statement_code
        return vm_code_list

    def _compileLet(self, statement: ET.Element) -> List[str]:
        assert statement.tag == "letStatement"
        expression_elem_list = statement.findall("expression")
        vm_code_list = []
        if len(expression_elem_list) == 1:
            var_name_elem = statement.find("identifier")
            exp_code = self._compileExpression(expression_elem_list[0])
            symbol = self._symbol_table.symbol(var_name_elem.text)
            vm_code_list = vm_code_list + exp_code
            segment = kind2Str(symbol.kind)
            vm_code_list.append(f"    pop {segment} {symbol.index}")
        elif len(expression_elem_list) == 2:
            index_code = self._compileExpression(expression_elem_list[0])
            exp_code = self._compileExpression(expression_elem_list[1])
            vm_code_list = vm_code_list + exp_code
            var_name_elem = statement.find("identifier")
            symbol = self._symbol_table.symbol(var_name_elem.text)
            assert symbol.type_ == "Array"
            segment = kind2Str(symbol.kind)
            vm_code_list.append(f"    push {segment} {symbol.index}")
            vm_code_list = vm_code_list + index_code
            vm_code_list.append( "    add")
            vm_code_list.append( "    pop pointer 1")
            vm_code_list.append( "    pop that 0")
        else:
            assert False 
        return vm_code_list

    def _compileIf(self, statement: ET.Element) -> List[str]:
        assert statement.tag == "ifStatement"
        statements_elem_list = statement.findall("statements")
        expression_elem = statement.find("expression")
        exp_code = self._compileExpression(expression_elem)
        vm_code_list = exp_code
        vm_code_list.append( "    not")
        if len(statements_elem_list) == 1:
            body_code = self._compileStatements(statements_elem_list[0])
            label = f"{self._subroutine_identifier}${self._label_index}"
            self._label_index += 1
            vm_code_list.append(f"    if-goto {label}")
            vm_code_list += body_code
            vm_code_list.append(f"label {label}")
        elif len(statements_elem_list) == 2:
            if_body_code = self._compileStatements(statements_elem_list[0])
            else_body_code = self._compileStatements(statements_elem_list[1])
            else_label = f"{self._subroutine_identifier}${self._label_index}"
            self._label_index += 1
            label = f"{self._subroutine_identifier}${self._label_index}"
            self._label_index += 1
            vm_code_list.append(f"    if-goto {else_label}")
            vm_code_list += if_body_code
            vm_code_list.append(f"    goto {label}")
            vm_code_list.append(f"label {else_label}")
            vm_code_list += else_body_code
            vm_code_list.append(f"label {label}")
        else:
            assert False
        return vm_code_list

    def _compileWhile(self, statement: ET.Element) -> List[str]:
        assert statement.tag == "whileStatement"
        statements_elem = statement.find("statements")
        expression_elem = statement.find("expression")
        exp_code = self._compileExpression(expression_elem)
        body_code = self._compileStatements(statements_elem)
        vm_code_list = []
        while_label = f"{self._subroutine_identifier}${self._label_index}"
        self._label_index += 1
        label = f"{self._subroutine_identifier}${self._label_index}"
        self._label_index += 1
        vm_code_list.append(f"label {while_label}")
        vm_code_list += exp_code
        vm_code_list.append( "    not")
        vm_code_list.append(f"    if-goto {label}")
        vm_code_list += body_code
        vm_code_list.append(f"    goto {while_label}")
        vm_code_list.append(f"label {label}")
        return vm_code_list

    def _compileDo(self, statement: ET.Element) -> List[str]:
        assert statement.tag == "doStatement"
        identifier_elem_list = statement.findall("identifier")
        expression_list_elem = statement.find("expressionList")
        vm_code_list = []
        n_args, exp_code = self._compileExpressionList(expression_list_elem)
        if len(identifier_elem_list) == 1:
            vm_code_list.append( "    push pointer 0")  # push this to stack 
            vm_code_list += exp_code
            vm_code_list.append(f"    call {self._class_identifier}.{identifier_elem_list[0].text} {n_args+1}")
        elif len(identifier_elem_list) == 2:
            class_name = identifier_elem_list[0].text
            symbol = self._symbol_table.symbol(class_name)
            subroutine_name = f"{class_name}.{identifier_elem_list[1].text}"
            if symbol != None:
                subroutine_name = f"{symbol.type_}.{identifier_elem_list[1].text}"
                segment = kind2Str(symbol.kind)
                vm_code_list.append(f"    push {segment} {symbol.index}") 
                n_args += 1
            vm_code_list += exp_code
            vm_code_list.append(f"    call {subroutine_name} {n_args}")
        else:
            assert False
        return vm_code_list

    def _compileReturn(self, statement: ET.Element) -> List[str]:
        assert statement.tag == "returnStatement"
        expression_elem = statement.find("expression")
        vm_code_list = []
        if expression_elem != None:
            exp_code = self._compileExpression(expression_elem)
            vm_code_list += exp_code
        vm_code_list.append("    return")
        return vm_code_list

    def _isUnaryTerm(self, term: ET.Element) -> bool:
        term_iter = term.iter()
        # skip term tag
        next(term_iter)
        first_elem = next(term_iter)
        return first_elem.tag == "symbol" and (first_elem.text == "-" or first_elem.text == "~")
    
    def _isArrayTerm(self, term: ET.Element) -> bool:
        term_iter = term.iter()
        # skip term tag
        next(term_iter)
        first_elem = next(term_iter)
        if first_elem.tag != "identifier":
            return False
        try:
            second_elem = next(term_iter)
            if second_elem.tag == "symbol" and second_elem.text == "[":
                return True
        except:
            return False 
        
    def _isSubroutineCall(self, term: ET.Element) -> Tuple[bool, Optional[str], Optional[str]]:
        term_iter = term.iter()
        # skip term tag
        next(term_iter)
        class_identifier_elem = next(term_iter)
        if class_identifier_elem.tag != "identifier":
            return False, None, None
        class_identifier = class_identifier_elem.text
        try:
            symbol_elem = next(term_iter)
            if symbol_elem.text != "." and symbol_elem.text != "(":
                return False, None, None
            if symbol_elem == '(':
                return True, None, class_identifier
            subroutine_identifier_elem = next(term_iter)
            assert subroutine_identifier_elem.tag == "identifier"
            return True, class_identifier, subroutine_identifier_elem.text
        except:
            return False, None, None

    def _isExpressionTerm(self, term: ET.Element) -> bool:
        identifier = term.find("identifier")
        symbol = term.find("symbol")
        return identifier == None and symbol != None and symbol.text == "("

    def _compileExpression(self, expression: ET.Element) -> List[str]:
        vm_code_list = []
        term_elem_list = expression.findall("term")
        op_elem_list = expression.findall("symbol")
        assert len(op_elem_list) + 1 == len(term_elem_list), f"Invalid expression {ET.tostring(expression)}"
        lhs_code = self._compileTerm(term_elem_list[0])
        vm_code_list = vm_code_list + lhs_code
        for i in range(1, len(term_elem_list)):
            rhs = term_elem_list[i]
            op = op_elem_list[i-1]
            rhs_code = self._compileTerm(rhs)
            vm_code_list = vm_code_list + rhs_code
            vm_code_list.append(f"    {self.op_map_vm_code.get(op.text)}") 
        return vm_code_list

    def _compileTerm(self, term: ET.Element) -> List[str]:
        vm_code_list = []
        if term.find("integerConstant") != None:
            text = term.find("integerConstant").text
            vm_code_list.append(f"    push constant {int(text)}")
        elif term.find("stringConstant") != None:
            text = term.find("stringConstant").text
            size = len(text)
            vm_code_list.append(f"    push constant {size}")
            vm_code_list.append(f"    call String.new 1")
            for c in text:
                vm_code_list.append(f"    push constant {ord(c)}")
                vm_code_list.append(f"    call String.appendChar 2")
        elif term.find("keyword") != None:
            text = term.find("keyword").text
            if text == "null" or text == "false":
                vm_code_list.append(f"    push constant 0")
            elif text == "true":
                vm_code_list.append(f"    push constant 0")
                vm_code_list.append(f"    not")
            else:
                assert text == "this"
                vm_code_list.append(f"    push pointer 0")
        elif self._isUnaryTerm(term):
            term_code_list = self._compileTerm(term.find("term"))
            vm_code_list = vm_code_list + term_code_list
            if term.find("symbol").text == "-":
                vm_code_list.append( "    neg")
            else:
                vm_code_list.append( "    not")
        elif self._isArrayTerm(term):
            index_code_list = self._compileExpression(term.find("expression"))
            var_name = term.find("identifier").text
            symbol = self._symbol_table.symbol(var_name)
            segment = kind2Str(symbol.kind)
            vm_code_list.append(f"    push {segment} {symbol.index}") # store addr of array
            vm_code_list = vm_code_list + index_code_list # compute index of array
            vm_code_list.append( "    add")
            vm_code_list.append( "    pop pointer 1") # store address of element of array
            vm_code_list.append( "    push that 0") # push elem of array to top stack
        elif self._isExpressionTerm(term):
            exp_code_list = self._compileExpression(term.find("expression"))
            vm_code_list = vm_code_list + exp_code_list
        else:
            subroutine_call, class_identifier, subroutine_identifier = \
                self._isSubroutineCall(term)
            if subroutine_call:
                symbol = self._symbol_table.symbol(class_identifier)
                subroutine_name = f"{class_identifier}.{subroutine_identifier}"
                n_args, exp_list_code = self._compileExpressionList(term.find("expressionList"))
                if symbol:
                    # object
                    segment = kind2Str(symbol.kind)
                    vm_code_list.append(f"    push {segment} {symbol.index}")
                    subroutine_name = f"{symbol.type_}.{subroutine_identifier}"
                    n_args = n_args + 1
                vm_code_list = vm_code_list + exp_list_code
                vm_code_list.append(f"    call {subroutine_name} {n_args}")
            else:
                # varName
                symbol = self._symbol_table.symbol(term.find("identifier").text)
                segment = kind2Str(symbol.kind)
                vm_code_list.append(f"    push {segment} {symbol.index}")
        return vm_code_list

    def _compileExpressionList(self, expression_list: ET.Element) -> Tuple[int, List[str]]:
        assert expression_list.tag == "expressionList"
        expression_elem_list = expression_list.findall("expression")
        vm_code_list = []
        for expression_elem in expression_elem_list:
            expression_vm_code = self._compileExpression(expression_elem)
            vm_code_list = vm_code_list + expression_vm_code
        return len(expression_elem_list), vm_code_list

    def _defineSubroutineArg(self, parameter_list: ET.Element):
        arg_type = ""
        arg_name = ""
        for child in parameter_list:
            if child.tag == "symbol":
                assert child.text == ","
                self._symbol_table.define(arg_name, arg_type, SymbolKind.ARGUMENT)
                print(f"defineSubroutineArg: Subroutine {self._class_identifier}.{self._subroutine_identifier}" +
                      f" arg {self._symbol_table.symbol(arg_name)}") 
                arg_type = ""
                arg_name = ""
            elif not arg_type:
                arg_type = child.text
            else:
                arg_name = child.text 
        if len(arg_type) > 0:
            self._symbol_table.define(arg_name, arg_type, SymbolKind.ARGUMENT)
            print(f"defineSubroutineArg: Subroutine {self._class_identifier}.{self._subroutine_identifier}" +
                  f" arg {self._symbol_table.symbol(arg_name)}")  

    def _defineSubroutineVar(self, subroutine: ET.Element):
        subrutine_body = subroutine.find("subroutineBody")
        var_dec_list = subrutine_body.findall("varDec")
        if not var_dec_list:
            return

        for var_dec in var_dec_list:
            var_dec_iter = var_dec.iter()
            # skip varDec
            next(var_dec_iter)
            # skip var
            next(var_dec_iter)
            type_elem = next(var_dec_iter)
            while True:
                try:
                    var_name_elem = next(var_dec_iter)
                except:
                    break
                if var_name_elem.tag != "identifier":
                    assert var_name_elem.tag == "symbol" and (var_name_elem.text == "," or var_name_elem.text == ";"), ET.tostring(var_name_elem)
                    continue
                self._symbol_table.define(var_name_elem.text, type_elem.text, SymbolKind.VAR)
                print(f"defineSubroutineVar: Subroutine {self._class_identifier}.{self._subroutine_identifier}" +
                      f" var {self._symbol_table.symbol(var_name_elem.text)}")

    def _defineClassVar(self, class_var_dec_list: List[ET.Element]):
        for class_var_dec in class_var_dec_list:
            kind, type_ = "", ""
            for child in class_var_dec:
                if child.tag == "symbol":
                    continue
                if not kind:
                    kind = child.text
                    kind = str2Kind(kind)
                elif not type_:
                    type_ = child.text
                else:
                    self._symbol_table.define(child.text, type_, kind)
                    print("defineClassVar: Get one symbol ", self._symbol_table.symbol(child.text))

    def close(self):
        self._file.flush()
        self._file.close()


def _test_term():
    parse_tree = ET.Element("class")
    class_elem = ET.Element("identifier")
    class_elem.text = "Main"
    parse_tree.append(class_elem)
    vm_writer = JackVMWriter(parse_tree, "") 
    
    # integerConstant
    data = """
<term>
    <integerConstant>42</integerConstant>
</term>
"""
    term = ET.fromstring(data)
    vm_code = vm_writer._compileTerm(term)
    for code in vm_code:
        print(code)

    # stringConstant
    data = """
<term>
    <stringConstant>hello world</stringConstant>
</term>
"""
    term = ET.fromstring(data)
    vm_code = vm_writer._compileTerm(term)
    for code in vm_code:
        print(code)

    # keywordConstant
    data = """
<term>
    <keyword>true</keyword>
</term>
"""
    term = ET.fromstring(data)
    vm_code = vm_writer._compileTerm(term)
    for code in vm_code:
        print(code)

    # varName
    vm_writer._symbol_table.define("i", "int", SymbolKind.ARGUMENT)
    data = """
<term>
    <identifier>i</identifier>
</term>
"""
    term = ET.fromstring(data)
    vm_code = vm_writer._compileTerm(term)
    for code in vm_code:
        print(code)

    # unaryOp term
    data = """
<term>
    <symbol>~</symbol>
    <term>
        <identifier>i</identifier>
    </term>
</term>
"""
    term = ET.fromstring(data)
    vm_code = vm_writer._compileTerm(term)
    for code in vm_code:
        print(code) 

def _test_expression():
    parse_tree = ET.Element("class")
    class_elem = ET.Element("identifier")
    class_elem.text = "Main"
    parse_tree.append(class_elem)
    vm_writer = JackVMWriter(parse_tree, "") 
    # -2+(3*arr[i-1])*extend
    data = """
<expression>
    <term>
        <symbol>-</symbol>
        <term>
            <integerConstant>2</integerConstant>
        </term>
    </term>
    <symbol>+</symbol>
    <term>
        <symbol>(</symbol>
        <expression>
            <term>
                <integerConstant>3</integerConstant>
            </term>
            <symbol>*</symbol>
            <term>
                <identifier>arr</identifier>
                <symbol>[</symbol>
                <expression>
                    <term>
                        <identifier>i</identifier>
                    </term>
                    <symbol>-</symbol>
                    <term>
                        <integerConstant>1</integerConstant>
                    </term>
                </expression>
                <symbol>]</symbol>
            </term>
        </expression>
        <symbol>)</symbol>
    </term>
    <symbol>*</symbol>
    <term>
        <identifier>extend</identifier>
    </term>
</expression>
"""
    expression = ET.fromstring(data)
    vm_writer._symbol_table.define("arr", "Array", SymbolKind.FIELD)
    vm_writer._symbol_table.define("i", "int", SymbolKind.VAR)
    vm_writer._symbol_table.define("extend", "int", SymbolKind.STATIC)
    vm_code = vm_writer._compileExpression(expression)
    for code in vm_code:
        print(code)
    
    # obj.comp(arr, i, j, false)*obj.comp(arr, j, i, true)<Foo.getMax(i, j)
    data = """
<expression>
    <term>
        <identifier>obj</identifier>
        <symbol>.</symbol>
        <identifier>comp</identifier>
        <symbol>(</symbol>
            <expressionList>
                <expression>
                    <term>
                        <identifier>arr</identifier>
                    </term>
                </expression>
                <symbol>,</symbol>
                <expression>
                    <term>
                        <identifier>i</identifier>
                    </term>
                </expression>
                <symbol>,</symbol>
                <expression>
                    <term>
                        <identifier>j</identifier>
                    </term>
                </expression>
                <symbol>,</symbol>
                <expression>
                    <term>
                        <keyword>false</keyword>
                    </term>
                </expression>
            </expressionList>
        <symbol>)</symbol>
    </term>
    <symbol>*</symbol>
    <term>
        <identifier>obj</identifier>
        <symbol>.</symbol>
        <identifier>comp</identifier>
        <symbol>(</symbol>
            <expressionList>
                <expression>
                    <term>
                        <identifier>arr</identifier>
                    </term>
                </expression>
                <symbol>,</symbol>
                <expression>
                    <term>
                        <identifier>j</identifier>
                    </term>
                </expression>
                <symbol>,</symbol>
                <expression>
                    <term>
                        <identifier>i</identifier>
                    </term>
                </expression>
                <symbol>,</symbol>
                <expression>
                    <term>
                        <keyword>true</keyword>
                    </term>
                </expression>
            </expressionList>
        <symbol>)</symbol>
    </term>
    <symbol>&lt;</symbol>
    <term>
        <identifier>Foo</identifier>
        <symbol>.</symbol>
        <identifier>getMax</identifier>
        <symbol>(</symbol>
            <expressionList>
                <expression>
                    <term>
                        <identifier>i</identifier>
                    </term>
                </expression>
                <symbol>,</symbol>
                <expression>
                    <term>
                        <identifier>j</identifier>
                    </term>
                </expression>
            </expressionList>
        <symbol>)</symbol>
    </term>
</expression>
"""
    vm_writer._symbol_table.define("obj", "Foo", SymbolKind.ARGUMENT)
    vm_writer._symbol_table.define("j", "int", SymbolKind.VAR)
    expression = ET.fromstring(data)
    vm_code = vm_writer._compileExpression(expression)
    for code in vm_code:
        print(code)

def _test_let():
    parse_tree = ET.Element("class")
    class_elem = ET.Element("identifier")
    class_elem.text = "Main"
    parse_tree.append(class_elem)
    vm_writer = JackVMWriter(parse_tree, "") 
    # let i = 0;
    data = """
<letStatement>
    <keyword>let</keyword>
    <identifier>i</identifier>
    <symbol>=</symbol>
    <expression>
        <term>
            <integerConstant>0</integerConstant>
        </term>
    </expression>
    <symbol>;</symbol>
</letStatement>
"""
    statement = ET.fromstring(data)
    vm_writer._symbol_table.define("i", "int", SymbolKind.VAR)
    vm_code = vm_writer._compileLet(statement)
    for code in vm_code:
        print(code)
    print("\n")

    # let arr[i] = i + 1;
    data = """
<letStatement>
    <identifier>arr</identifier>
    <symbol>[</symbol>
    <expression>
        <term>
            <identifier>i</identifier>
        </term>
    </expression>
    <symbol>]</symbol>
    <symbol>=</symbol>
    <expression>
        <term>
            <identifier>i</identifier>
        </term>
        <symbol>+</symbol>
        <term>
            <integerConstant>1</integerConstant>
        </term>
    </expression> 
    <symbol>;</symbol>
</letStatement>
"""
    statement = ET.fromstring(data)
    vm_writer._symbol_table.define("arr", "Array", SymbolKind.FIELD)
    vm_code = vm_writer._compileLet(statement)
    for code in vm_code:
        print(code)
    print("\n")


if __name__ == "__main__":
    import sys
    test_dict: Dict[str, Callable[[], None]] = {
        "term":         _test_term,
        "expression":   _test_expression,
        "let":          _test_let,
    }
    test_fn = test_dict.get(sys.argv[1])
    test_fn()
