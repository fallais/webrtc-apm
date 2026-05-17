#include "crnnoise.h"

#include <rnnoise.h>
#include <stdlib.h>

struct crnnoise {
    DenoiseState *st;
};

crnnoise_t *crnnoise_create(void) {
    crnnoise_t *p = (crnnoise_t *)malloc(sizeof(crnnoise_t));
    if (!p) return NULL;
    p->st = rnnoise_create(NULL);
    if (!p->st) {
        free(p);
        return NULL;
    }
    return p;
}

void crnnoise_destroy(crnnoise_t *p) {
    if (!p) return;
    if (p->st) rnnoise_destroy(p->st);
    free(p);
}

int crnnoise_frame_size(void) {
    return rnnoise_get_frame_size();
}

float crnnoise_process_frame(crnnoise_t *p, float *out, const float *in) {
    if (!p || !p->st) return 0.0f;
    return rnnoise_process_frame(p->st, out, in);
}
