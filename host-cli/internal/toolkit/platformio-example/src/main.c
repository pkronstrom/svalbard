#include <stdio.h>
#include "driver/gpio.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"

// Set this to the GPIO for your board's ordinary LED. Some boards instead
// have an addressable RGB LED, requiring a board-specific driver.
#define LED_GPIO GPIO_NUM_2

void app_main(void) {
    gpio_reset_pin(LED_GPIO);
    gpio_set_direction(LED_GPIO, GPIO_MODE_OUTPUT);
    for (int level = 0; ; level ^= 1) {
        gpio_set_level(LED_GPIO, level);
        printf("Svalbard offline example: GPIO %d = %d\n", LED_GPIO, level);
        vTaskDelay(pdMS_TO_TICKS(500));
    }
}
