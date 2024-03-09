PTE_V = 1 << 0 
PTE_R = 1 << 1
PTE_W = 1 << 2
PTE_X = 1 << 3
PTE_U = 1 << 4 
PTE_G = 1 << 5
PTE_A = 1 << 6

permission_map = {
    PTE_V: "V",
    PTE_R: "R",
    PTE_W: "W",
    PTE_X: "X",
    PTE_U: "U",
    PTE_G: "G",
    PTE_A: "A",
}

def parse_permission(pte):
    assert pte & PTE_V == 1
    ans = ""
    for i in range(1, 7):
        mask = 1 << i
        if pte & mask:
            ans = ans + permission_map[mask]
        else:
            ans = ans + "-"
    return ans

def generate_va(index1, index2, index3):
    return ((index1 << 18) + (index2 << 9) + index3) << 12

def transform_markdown(arr):
    ans = "|idx|va|perm|pa|\n|:-:|:-:|:-:|\n"
    idx = 0
    for elem in arr:
        ans = ans + "|{}|{}|{}|{}|\n".format(idx, elem[0], elem[1], elem[2])
        idx += 1
    return ans

def main():
    index1, index2 = 0, 0
    ans = []
    while True:
        try:
            line = input()
            if not line.startswith(".."):
                continue
            depth = line.count("..")
            line = line[2*depth+depth-1:]
            items = line.split()
            assert len(items) == 5
            if depth == 1:
                index1 = int(items[0][:-1])
            elif depth == 2:
                index2 = int(items[0][:-1])
            else:
                assert depth == 3
                index3 = int(items[0][:-1])
                pte = int(items[2], base=16)
                permission = parse_permission(pte)
                va = generate_va(index1, index2, index3)
                # print("DEBUG: index1={} index2={} index3={} pte={} perm={} va={}".format(index1, index2, index3, pte, permission, va))
                ans.append([hex(va), permission, hex(int(items[-1], base=16))]) # va perm pa

        except EOFError:
            break
        except:
            raise
    print(transform_markdown(ans))

if __name__ == "__main__":
    main()
