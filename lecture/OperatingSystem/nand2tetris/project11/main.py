from jack_tokenizer import JackTokenizer
from jack_compiler_engine import JackCompilerEngine
from jack_vm_writer import JackVMWriter
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
        output_name = os.path.splitext(os.path.basename(file))[0] + ".vm"
        output_file = os.path.join(output_directory, output_name)
        vm_writer = JackVMWriter(root, output_file)
        vm_writer.run()
        vm_writer.close()


if __name__ == "__main__":
    if len(sys.argv) != 3:
        usage()
    else:
        main()
