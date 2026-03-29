# Solution


## Q1 & 

```c
__cs149_vec_float x;
__cs149_vec_int exp;
__cs149_vec_int zero = _cs149_vset_int(0);
__cs149_vec_int one = _cs149_vset_int(1);
for (int i = 0; i < N; i += VECTOR_WIDTH) {
  __cs149_mask maskAll = _cs149_init_ones(min(VECTOR_WIDTH, N - i));
  _cs149_vload_float(x, values + i, maskAll);
  _cs149_vload_int(exp, exponents + i, maskAll);

  __cs149_vec_float result = _cs149_vset_float(1.f);
  __cs149_mask maskGtZero;
  _cs149_vgt_int(maskGtZero, exp, zero, maskAll);
  while (_cs149_cntbits(maskGtZero) > 0) {
    _cs149_vmult_float(result, result, x, maskGtZero);
    _cs149_vsub_int(exp, exp, one, maskGtZero);
    _cs149_vgt_int(maskGtZero, exp, zero, maskGtZero);
  }
  __cs149_vec_float clamp = _cs149_vset_float(9.999999f);
  __cs149_mask maskGtClamp;
  _cs149_vgt_float(maskGtClamp, result, clamp, maskAll);
  _cs149_vmove_float(result, clamp, maskGtClamp);
  _cs149_vstore_float(output + i, result, maskAll);
}
```

|vector width|total vector instructions|vector utilization|
|---|---|---|
|2|167514|80.4%|
|4|97070|72.8%|
|8|52876|68.9%|

## Q3

```c
__cs149_vec_float sum = _cs149_vset_float(0.f);
__cs149_vec_float x;
__cs149_mask ones = _cs149_init_ones();
for (int i=0; i<N; i+=VECTOR_WIDTH) {
  _cs149_vload_float(x, values + i, ones);
  _cs149_vadd_float(sum, sum, x, ones); 
}

int times = VECTOR_WIDTH / 2 - 1;
for (int i = 0; i < times; i++) {
  __cs149_vec_float temp;
  _cs149_hadd_float(temp, sum);
  _cs149_interleave_float(sum, temp);
}

float result[VECTOR_WIDTH];
_cs149_vstore_float(result, sum, ones);

return result[0];
```