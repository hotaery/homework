#include <immintrin.h>
#include <math.h>
#include <stdio.h>
#include <stdlib.h>

static const float kThreshold = 0.00001f;

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


