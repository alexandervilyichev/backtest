import json
import matplotlib.pyplot as plt
import numpy as np
from mpl_toolkits.mplot3d import Axes3D
import matplotlib.tri as tri

def main():
    # Read mesh data
    with open('grid_search_results.json', 'r') as f:
        results = json.load(f)
    
    # Extract parameters and profits
    min_lengths = np.array([r['X'] for r in results])
    max_lengths = np.array([r['Y'] for r in results])
    profits = np.array([r['profit'] for r in results])
    
    # Create 3D surface plot
    fig = plt.figure(figsize=(12, 8))
    ax = fig.add_subplot(111, projection='3d')
    
    # Create a triangulated surface
    triang = tri.Triangulation(min_lengths, max_lengths)
    surf = ax.plot_trisurf(triang, profits, cmap='viridis', edgecolor='none')
    
    ax.set_xlabel('X')
    ax.set_ylabel('Y')
    ax.set_zlabel('Profit')
    ax.set_title(' Grid Search Results')
    fig.colorbar(surf, ax=ax, shrink=0.5, aspect=5)
    
    # Save plot
    plt.savefig('grid_search.png')
    plt.close()
    
    print("3D visualization saved as 'grid_search.png'")

if __name__ == '__main__':
    main()
