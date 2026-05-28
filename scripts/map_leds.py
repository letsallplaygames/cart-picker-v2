import time
import argparse
from rpi_ws281x import PixelStrip, Color
import config

DEFAULT_LED_COUNT = 300

def clear_strip(strip):
    """Turns off all LEDs on the strip."""
    for i in range(strip.numPixels()):
        strip.setPixelColor(i, Color(0, 0, 0))
    strip.show()

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Interactive LED mapping tool')
    parser.add_argument(
        '--led-count',
        type=int,
        default=DEFAULT_LED_COUNT,
        help=f'Number of LEDs to map (default: {DEFAULT_LED_COUNT})'
    )
    parser.add_argument(
        '--start-led',
        type=int,
        default=0,
        help='Starting LED index (default: 0)'
    )
    args = parser.parse_args()

    if args.led_count <= 0:
        raise ValueError('LED count must be greater than 0')

    # Initialize the LED strip using your config.py settings
    strip = PixelStrip(
        args.led_count,
        config.LED_PIN, 
        config.LED_FREQ_HZ, 
        config.LED_DMA, 
        config.LED_INVERT, 
        config.LED_BRIGHTNESS, 
        config.LED_CHANNEL
    )
    strip.begin()
    
    current_led = max(0, min(args.start_led, args.led_count - 1))
    max_leds = strip.numPixels()
    
    print("\n--- Interactive LED Mapping Tool ---")
    print(f"Total LEDs configured for this run: {max_leds}")
    print("Controls:")
    print("  [Enter]  : Light up the NEXT LED")
    print("  'b'      : Go BACK to the previous LED")
    print("  [Number] : Jump straight to a specific LED index")
    print("  'q'      : Quit and turn off lights\n")
    
    try:
        while True:
            # Wrap around if we exceed the max LED count
            if current_led >= max_leds:
                print("Reached the end of the strip. Looping back to 0.")
                current_led = 0
            
            clear_strip(strip)
            
            # Light up the target LED (Bright White)
            # You can change Color(255, 255, 255) if you prefer a different color
            strip.setPixelColor(current_led, Color(255, 255, 255))
            strip.show()
            
            # Wait for your input
            command = input(f"[*] LED {current_led} is ON. Command: ").strip().lower()
            
            if command == 'q':
                break
            elif command == 'b':
                # Go back, but don't drop below 0
                current_led = max(0, current_led - 1)
            elif command.isdigit():
                # Jump to a specific number
                new_led = int(command)
                if 0 <= new_led < max_leds:
                    current_led = new_led
                else:
                    print(f"  -> Invalid index! Must be between 0 and {max_leds - 1}.")
            else:
                # Default behavior: go to the next LED
                current_led += 1
                
    except KeyboardInterrupt:
        # Handle CTRL+C gracefully
        pass
    finally:
        clear_strip(strip)
        print("\nExiting mapping tool. Lights off.")