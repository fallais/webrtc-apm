// C++ wrapper exposing a C ABI over webrtc-audio-processing 2.x.
//
// 2.x carries forward the 1.x API shape (AudioProcessingBuilder +
// AudioProcessing::Config + StreamConfig-based ProcessStream) with
// improved AEC3, NS and transient-suppression tunings inherited from a
// more recent libwebrtc snapshot.
//
// VAD is intentionally not exposed: the public voice_detection getter
// was removed in 1.x and was not reinstated in 2.x. VAD still runs
// internally (AGC2 / NS / AEC3 are all VAD-driven), but the verdict is
// not stably consumable from outside. Callers needing a VAD verdict
// should either enable rnnoise (whose ProcessFrame returns a voice
// probability) or use a standalone detector downstream of APM.

#include "capm.h"

#include <modules/audio_processing/include/audio_processing.h>

#include <cstdint>
#include <cstdlib>
#include <memory>

using webrtc::AudioProcessing;
using webrtc::AudioProcessingBuilder;
using webrtc::StreamConfig;

struct capm {
    std::unique_ptr<AudioProcessing> apm;
    int sample_rate_hz;
    int num_channels;
};

extern "C" {

capm_t *capm_create(const capm_config_t *cfg) {
    if (!cfg) return nullptr;

    std::unique_ptr<AudioProcessing> apm(AudioProcessingBuilder().Create());
    if (!apm) return nullptr;

    AudioProcessing::Config config;

    if (cfg->enable_aec) {
        config.echo_canceller.enabled = true;
        config.echo_canceller.mobile_mode = false;
    }
    if (cfg->enable_ns) {
        config.noise_suppression.enabled = true;
        config.noise_suppression.level =
            static_cast<AudioProcessing::Config::NoiseSuppression::Level>(cfg->ns_level);
    }
    if (cfg->enable_agc) {
        config.gain_controller1.enabled = true;
        config.gain_controller1.mode =
            static_cast<AudioProcessing::Config::GainController1::Mode>(cfg->agc_mode);
    }
    apm->ApplyConfig(config);

    capm_t *p = new capm{std::move(apm), cfg->sample_rate_hz, cfg->num_channels};
    return p;
}

void capm_destroy(capm_t *p) {
    delete p;
}

int capm_process_stream(capm_t *p, int16_t *samples, int num_samples) {
    if (!p || !p->apm || !samples) return -1;
    if (num_samples <= 0) return -2;

    StreamConfig in_cfg(p->sample_rate_hz, p->num_channels);
    StreamConfig out_cfg(p->sample_rate_hz, p->num_channels);
    if (num_samples != static_cast<int>(in_cfg.num_samples())) return -3;

    /* In-place processing: src == dst is supported. */
    return p->apm->ProcessStream(samples, in_cfg, out_cfg, samples);
}

int capm_process_reverse_stream(capm_t *p, int16_t *samples, int num_samples) {
    if (!p || !p->apm || !samples) return -1;
    if (num_samples <= 0) return -2;

    StreamConfig in_cfg(p->sample_rate_hz, p->num_channels);
    StreamConfig out_cfg(p->sample_rate_hz, p->num_channels);
    if (num_samples != static_cast<int>(in_cfg.num_samples())) return -3;

    return p->apm->ProcessReverseStream(samples, in_cfg, out_cfg, samples);
}

} /* extern "C" */
