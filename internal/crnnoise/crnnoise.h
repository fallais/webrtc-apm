#ifndef WEBRTC_APM_CRNNOISE_H
#define WEBRTC_APM_CRNNOISE_H

#ifdef __cplusplus
extern "C" {
#endif

typedef struct crnnoise crnnoise_t;

/* Returns NULL on failure. */
crnnoise_t *crnnoise_create(void);

/* Safe on NULL. */
void crnnoise_destroy(crnnoise_t *p);

/* Returns rnnoise's fixed frame size in samples (480 at 48 kHz). */
int crnnoise_frame_size(void);

/* Denoise one frame; returns the voice-activity probability in [0,1]. */
float crnnoise_process_frame(crnnoise_t *p, float *out, const float *in);

#ifdef __cplusplus
}
#endif

#endif /* WEBRTC_APM_CRNNOISE_H */
