# ESP-IDF snippets for the drive's ESP32 example

ESP-IDF 6.1 in C, through PlatformIO (espressif32 7.1.3). Each snippet goes in
`src/main.c` of a copy of the drive's example and builds with
`sh <folder>/scripts/pio.sh <project> run`. Only components that ship with
ESP-IDF are available: no Arduino, no `lib_deps`.

## Log instead of printf

```c
#include "esp_log.h"
static const char *TAG = "app";
// in a function:
ESP_LOGI(TAG, "reading %d", value);   // ESP_LOGW, ESP_LOGE for warnings, errors
```

## Read a button (to ground, internal pull-up)

```c
#include "driver/gpio.h"
#define BUTTON_GPIO GPIO_NUM_0

void button_setup(void) {
    gpio_reset_pin(BUTTON_GPIO);
    gpio_set_direction(BUTTON_GPIO, GPIO_MODE_INPUT);
    gpio_set_pull_mode(BUTTON_GPIO, GPIO_PULLUP_ONLY);
}

int button_pressed(void) {
    return gpio_get_level(BUTTON_GPIO) == 0;
}
```

## Dim an LED with PWM (LEDC)

```c
#include "driver/ledc.h"
#define PWM_GPIO GPIO_NUM_2

void pwm_setup(void) {
    ledc_timer_config_t timer = {
        .speed_mode = LEDC_LOW_SPEED_MODE, .timer_num = LEDC_TIMER_0,
        .duty_resolution = LEDC_TIMER_10_BIT, .freq_hz = 5000, .clk_cfg = LEDC_AUTO_CLK,
    };
    ledc_timer_config(&timer);
    ledc_channel_config_t channel = {
        .speed_mode = LEDC_LOW_SPEED_MODE, .channel = LEDC_CHANNEL_0, .timer_sel = LEDC_TIMER_0,
        .gpio_num = PWM_GPIO, .duty = 0, .hpoint = 0,
    };
    ledc_channel_config(&channel);
}

void pwm_set(int duty) {   // 0..1023 at 10 bits
    ledc_set_duty(LEDC_LOW_SPEED_MODE, LEDC_CHANNEL_0, duty);
    ledc_update_duty(LEDC_LOW_SPEED_MODE, LEDC_CHANNEL_0);
}
```

## Read an analog input (ADC one-shot)

```c
#include "esp_adc/adc_oneshot.h"
static adc_oneshot_unit_handle_t adc;

void adc_setup(void) {
    adc_oneshot_unit_init_cfg_t unit = { .unit_id = ADC_UNIT_1 };
    adc_oneshot_new_unit(&unit, &adc);
    adc_oneshot_chan_cfg_t chan = { .atten = ADC_ATTEN_DB_12, .bitwidth = ADC_BITWIDTH_DEFAULT };
    adc_oneshot_config_channel(adc, ADC_CHANNEL_6, &chan);   // GPIO34 on the ESP32
}

int adc_read(void) {
    int raw = 0;
    adc_oneshot_read(adc, ADC_CHANNEL_6, &raw);   // 0..4095
    return raw;
}
```

## Wait without blocking other tasks

```c
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
vTaskDelay(pdMS_TO_TICKS(100));   // 100 ms
```
