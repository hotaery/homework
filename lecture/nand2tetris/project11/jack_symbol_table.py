from enum import Enum, auto
from typing import Dict, Optional


class SymbolKind(Enum):
    STATIC  = auto()
    FIELD   = auto()
    ARGUMENT= auto()
    VAR     = auto()


def str2Kind(kind: str) -> SymbolKind:
    if kind == 'static':
        return SymbolKind.STATIC
    elif kind == 'field':
        return SymbolKind.FIELD
    elif kind == 'var':
        return SymbolKind.VAR

def kind2Str(kind: SymbolKind) -> str:
    if kind == SymbolKind.STATIC:
        return "static"
    elif kind == SymbolKind.FIELD:
        return "this"
    elif kind == SymbolKind.ARGUMENT:
        return "argument"
    else:
        return "local"


class Symbol(object):
    def __init__(self, name: str, type_: str, kind: SymbolKind, index: int):
        self._name = name
        self._type = type_
        self._kind = kind
        self._index = index

    @property
    def name(self) -> str:
        return self._name 
    
    @property
    def type_(self) -> str:
        return self._type
    
    @property
    def kind(self) -> SymbolKind:
        return self._kind
    
    @property
    def index(self) -> int:
        return self._index

    def __str__(self) -> str:
        fmt = "{{{name} {type} {kind} {index}}}"
        return fmt.format(name=self._name, type=self._type, kind=self._kind.name, index=self._index)


class JackSymbolTable(object):

    def __init__(self, class_name: str):
        self._class_scope_table: Dict[str, Symbol] = {}
        self._method_scope_table: Dict[str, Symbol] = {}
        self._class_name = class_name
        self._method_name = ""

    def startSubroutine(self, method_name: str):
        self._method_scope_table.clear()
        self._method_name = method_name

    def define(self, name: str, type: str, kind: SymbolKind):
        cnt = self.varCount(kind)
        if kind == SymbolKind.STATIC or kind == SymbolKind.FIELD:
            assert not name in self._class_scope_table, f"{name}"
            self._class_scope_table[name] = Symbol(name, type, kind, cnt)
        else:
            assert not name in self._method_scope_table
            self._method_scope_table[name] = Symbol(name, type, kind, cnt)

    def varCount(self, kind: SymbolKind) -> int:
        if kind == SymbolKind.STATIC or kind == SymbolKind.FIELD:
            return self._classVarCound(kind)
        else:
            return self._methodVarCound(kind) 

    def _classVarCound(self, kind: SymbolKind) -> int:
        assert kind == SymbolKind.STATIC or kind == SymbolKind.FIELD
        cnt = 0
        for _, v in self._class_scope_table.items():
            if v.kind == kind:
                cnt += 1
        return cnt 

    def _methodVarCound(self, kind: SymbolKind) -> int:
        assert kind == SymbolKind.ARGUMENT or kind == SymbolKind.VAR
        cnt = 0
        for _, v in self._method_scope_table.items():
            if v.kind == kind:
                cnt += 1
        return cnt

    def symbol(self, name: str) -> Optional[Symbol]:
        if name in self._method_scope_table:
            return self._method_scope_table.get(name)
        elif name in self._class_scope_table:
            return self._class_scope_table.get(name)
        else:
            print(f"SymbolTable: {self._class_name} symbol {name} not exist")
            return None

    def kindOf(self, name: str) -> SymbolKind:
        return self.symbol(name).kind
    
    def typeOf(self, name: str) -> str:
        return self.symbol(name).type_
    
    def indexOf(self, name: str) -> int:
        return self.symbol(name).index
    
    @property
    def class_scope_table(self) -> Dict[str, Symbol]:
        return self._class_scope_table
    
    @property
    def method_scope_table(self) -> Dict[str, symbol]:
        return self._method_scope_table
    
    @property
    def class_name(self) -> str:
        return self._class_name
    
    @property
    def method_name(self) -> str:
        return self._method_name

    def inc_arg_index(self):
        for _, symbol in self._method_scope_table.items():
            if symbol.kind == SymbolKind.ARGUMENT:
                symbol._index += 1

    def __str__(self) -> str:
        fmt = "{{{class_name}.{method_name} static:{static_cnt} field:{field_cnt} arg:{arg_cnt} var:{var_cnt}}}"
        return fmt.format(class_name=self._class_name, method_name=self._method_name,
                          static_cnt=self.varCount(SymbolKind.STATIC),
                          field_cnt=self.varCount(SymbolKind.FIELD),
                          arg_cnt=self.varCount(SymbolKind.ARGUMENT),
                          var_cnt=self.varCount(SymbolKind.VAR))


def _test():
    symbol_table = JackSymbolTable("Student")
    # class data member
    symbol_table.define("_global_id", "int", SymbolKind.STATIC)
    symbol_table.define("_id", "int", SymbolKind.FIELD)
    symbol_table.define("_age", "int", SymbolKind.FIELD)
    symbol_table.define("_name", "Array", SymbolKind.FIELD)
    symbol_table.define("_school", "School", SymbolKind.FIELD)
    print(symbol_table)

    for _, info in symbol_table.class_scope_table.items():
        print(info)

    symbol_table.startSubroutine("setAge")
    symbol_table.define("age", "int", SymbolKind.ARGUMENT)
    symbol_table.define("old_age", "int", SymbolKind.VAR)
    for _, info in symbol_table.method_scope_table.items():
        print(info)
    symbol = symbol_table.symbol("old_age")
    print(symbol)
    symbol = symbol_table.symbol("_age")
    print(symbol)

    symbol_table.startSubroutine("getAge")
    for _, info in symbol_table.method_scope_table.items():
        print(info)
    symbol = symbol_table.symbol("_age")
    print(symbol)

if __name__ == "__main__":
    _test()
