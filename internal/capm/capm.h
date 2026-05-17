#ifndef WEBRTC_APM_CAPM_H
#define WEBRTC_APM_CAPM_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct capm capm_t;

typedef struct {
    int sample_rate_hz; /* 8000, 16000, 32000 or 48000 */
    int num_channels;   /* 1 or 2                      */

    int enable_aec;
    int enable_ns;
    int ns_level; /* 0=Low, 1=Moderate, 2=High, 3=VeryHigh */
    int enable_agc;
    int agc_mode; /* 0=AdaptiveAnalog, 1=AdaptiveDigital, 2=FixedDigital */
} capm_config_t;

/* Returns NULL on failure. */
capm_t *capm_create(const capm_config_t *cfg);

/* Safe on NULL. */
void capm_destroy(capm_t *p);

/* Process one 10ms near-end frame in place (src == dst is allowed). */
int capm_process_stream(capm_t *p, int16_t *samples, int num_samples);

/* Provide the far-end reference frame for AEC (processed in place). */
int capm_process_reverse_stream(capm_t *p, int16_t *samples, int num_samples);

#ifdef __cplusplus
}
#endif

#endif /* WEBRTC_APM_CAPM_H */
