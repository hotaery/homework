from jack_tokenizer import JackTokenizer
from jack_compiler_engine import JackCompilerEngine
from xml.etree import ElementTree as ET
import sys
import os

def usage():
    print("Usage: JackCompiler <file_or_directory> <output directory>")

def main():
    file_or_directory = sys.argv[1]
    output_directory = sys.argv[2]

    jack_file_list = []
    if os.path.isfile(file_or_directory):
        jack_file_list.append(file_or_directory)
    else:
        for file in os.listdir(file_or_directory):
            if os.path.splitext(file)[1] == '.jack':
                jack_file_list.append(os.path.join(file_or_directory, file))
    
    for file in jack_file_list:
        print(f"start compileing {file}...")
        tokenizer = JackTokenizer(file)
        engine = JackCompilerEngine(iter(tokenizer))
        root = engine.parse()
        tree = ET.ElementTree(root)
        ET.indent(tree)
        output_name = os.path.splitext(os.path.basename(file))[0] + ".xml"
        output_file = os.path.join(output_directory, output_name)
        tree.write(output_file, short_empty_elements=False)


if __name__ == "__main__":
    if len(sys.argv) != 3:
        usage()
    else:
        main()
