# Solution

## Q1

| 计算方法 | 耗时 (ms) | 加速比 |
| --- | --- | --- |
| sqrtSerial | 512.584 | - |
| sqrt_ispc | 110.347 | 4.65x |
| sqrt_ispc_tasks | 6.73 | 76.15x |

## Q2

选择迭代次数最大的值作为values的初始值，同时values的值保持一致，这样在`sqrt_ispc`中就没有无效迭代，这里我选择2.998

| 计算方法 | 耗时 (ms) | 加速比 |
| --- | --- | --- |
| sqrtSerial | 983.888 | - |
| sqrt_ispc | 167.998 | 5.86x |
| sqrt_ispc_tasks | 9.686 | 101.57x |

## Q3

如果需要最小加速比的话，由于使用的是avx2指令集，因此只要每个向量（8个32位浮点数）中都包含一个最大迭代次数的值，同时其余7个都是迭代次数最小的，那么就可以达到最小加速比了。这里我选择2.998f作为迭代次数最大的值，1.001f作为迭代次数最小的值。

| 计算方法 | 耗时 (ms) | 加速比 |
| --- | --- | --- |
| sqrtSerial | 130.610 | - |
| sqrt_ispc | 162.977 | 0.8x |
| sqrt_ispc_tasks | 9.997 | 13.06x |

## Q4 

```c
void sqrt_avx(
    int N,
    float initialGuess,
    float values[],
    float output[]
) {
    __m256 initialGuessVec = _mm256_set1_ps(initialGuess);
    __m256 thresholdVec = _mm256_set1_ps(kThreshold);

    for (int i = 0; i < N; i += 8) {
        __m256 xVec = _mm256_loadu_ps(&values[i]);
        __m256 guessVec = initialGuessVec;

        while (true) {
            __m256 guessSquaredVec = _mm256_mul_ps(guessVec, guessVec);
            __m256 errorVec = _mm256_sub_ps(_mm256_mul_ps(guessSquaredVec, xVec), _mm256_set1_ps(1.0f));
            // 将符号位设置位0
            __m256 absErrorVec = _mm256_andnot_ps(_mm256_set1_ps(-0.0f), errorVec); // fabs

            // _mm256_cmp_ps返回一个掩码，表示每个元素的比较结果，如果所有元素都小于等于阈值，则掩码为0
            // _mm256_movemask_ps获取每个元素的符号位
            if (_mm256_movemask_ps(_mm256_cmp_ps(absErrorVec, thresholdVec, _CMP_GT_OQ)) == 0) {
                break;
            }

            __m256 guessCubedVec = _mm256_mul_ps(guessSquaredVec, guessVec);
            __m256 newGuessVec = _mm256_mul_ps(_mm256_sub_ps(_mm256_mul_ps(_mm256_set1_ps(3.0f), guessVec), _mm256_mul_ps(xVec, guessCubedVec)), _mm256_set1_ps(0.5f));
            guessVec = newGuessVec;
        }

        __m256 resultVec = _mm256_mul_ps(xVec, guessVec);
        _mm256_storeu_ps(&output[i], resultVec);
    } 
}
```

| 计算方法 | 耗时 (ms) | 加速比 |
| --- | --- | --- |
| sqrtSerial | 540.508 | - |
| sqrt_ispc | 110.443 | 4.89x |
| sqrt_ispc_tasks | 6.881 | 78.00x |
| sqrt_avx | 85.179 | 6.30x |