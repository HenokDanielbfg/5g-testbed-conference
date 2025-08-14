#!/usr/bin/env python3
"""
Script to generate multiple free5gc UE YAML configuration files
with different SUPI values based on a template.
"""

import yaml
import os
import random
from typing import Dict, Any

def load_template(template_file: str) -> Dict[str, Any]:
    """Load the template YAML file."""
    with open(template_file, 'r') as file:
        return yaml.safe_load(file)

def generate_supi(base_imsi: str, ue_number: int) -> str:
    """
    Generate a new SUPI based on the base IMSI and UE number.
    
    Args:
        base_imsi: Base IMSI (e.g., 'imsi-208930000000021')
        ue_number: UE number to append/modify
    
    Returns:
        New SUPI string
    """
    # Extract the numeric part of the IMSI
    imsi_prefix = base_imsi.split('-')[0]  # 'imsi'
    imsi_number = base_imsi.split('-')[1]  # '208930000000021'
    
    # Replace the last few digits with the UE number (zero-padded)
    # Keep the MCC (208) and MNC (93) intact, modify the subscriber number part
    mcc_mnc = imsi_number[:5]  # '20893'
    
    # Generate new subscriber number (10 digits total for the remaining part)
    new_subscriber_number = f"{ue_number:010d}"
    
    new_imsi = f"{imsi_prefix}-{mcc_mnc}{new_subscriber_number}"
    return new_imsi

def generate_ue_files(template_file: str, num_ues: int, output_dir: str = "ue_configs", 
                     start_number: int = 1):
    """
    Generate multiple UE configuration files.
    
    Args:
        template_file: Path to the template YAML file
        num_ues: Number of UE files to generate
        output_dir: Directory to save the generated files
        start_number: Starting UE number
    """
    # Create output directory if it doesn't exist
    os.makedirs(output_dir, exist_ok=True)
    
    # Load the template
    template_config = load_template(template_file)
    base_supi = template_config['supi']
    
    print(f"Generating {num_ues} UE configuration files...")
    print(f"Base SUPI: {base_supi}")
    print(f"Output directory: {output_dir}")
    print("-" * 50)
    
    for i in range(start_number, start_number + num_ues):
        # Create a copy of the template
        ue_config = template_config.copy()
        
        # Generate new SUPI
        new_supi = generate_supi(base_supi, i)
        ue_config['supi'] = new_supi

        # Generate random x, y coordinates for phyLocation and work (bounded by [0, 170])
        if 'phyLocation' in ue_config:
            ue_config['x'] = random.randint(0, 170)
            ue_config['y'] = random.randint(0, 170)
        if 'work' in ue_config:
            ue_config['x1'] = random.randint(0, 170)
            ue_config['y1'] = random.randint(0, 170)
        
        # Generate output filename
        output_filename = f"free5gc-ue{i:02d}.yaml"
        output_path = os.path.join(output_dir, output_filename)
        
        # Write the new configuration file
        with open(output_path, 'w') as file:
            # Write comments at the top
            file.write("# IMSI number of the UE. IMSI = [MCC|MNC|MSISDN] (In total 15 digits)\n")
            file.write(f"supi: '{new_supi}'\n")
            file.write("# Mobile Country Code value of HPLMN\n")
            
            # Write the rest of the configuration (excluding supi since we already wrote it)
            config_without_supi = {k: v for k, v in ue_config.items() if k != 'supi'}
            yaml.dump(config_without_supi, file, default_flow_style=False, sort_keys=False)
        
        print(f"Generated: {output_filename} with SUPI: {new_supi}")
    
    print("-" * 50)
    print(f"Successfully generated {num_ues} UE configuration files in '{output_dir}' directory")

def main():
    """Main function with example usage."""
    # Configuration
    template_file = "free5gc-ue21.yaml"  # Your template file
    num_ues = 79  # Number of UE files to generate
    output_dir = "ue_configs"  # Output directory
    start_number = 22  # Starting UE number
    
    # Check if template file exists
    if not os.path.exists(template_file):
        print(f"Error: Template file '{template_file}' not found!")
        print("Please make sure the template file is in the current directory.")
        return
    
    try:
        generate_ue_files(template_file, num_ues, output_dir, start_number)
    except Exception as e:
        print(f"Error generating UE files: {e}")

if __name__ == "__main__":
    main()