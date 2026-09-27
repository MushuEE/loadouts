import { useEffect, useState } from 'react';
import { categoryIcon } from '../lib/display';

/**
 * An item's picture, or its category glyph when it has none.
 *
 * The fallback is the main requirement, not an edge case: most catalogue items have no
 * image, and retailer images are hotlinked, so they can 404 or be blocked at any time. A
 * broken-image icon would be worse than the glyph it replaced.
 *
 * The box is a fixed square whether or not the image has loaded, so thumbnails arriving
 * after paint never reflow the slot grid. Product shots are almost always on white and
 * glare against the dark UI, so they sit on a light tile that reads as a deliberate card
 * rather than a hole punched in the page.
 */
export function ItemThumb({
  imageUrl,
  category,
  size = 'w-8 h-8',
  iconSize = 'w-4 h-4',
  className = '',
}: {
  imageUrl?: string;
  category: string;
  /** Tailwind width/height classes for the square. */
  size?: string;
  iconSize?: string;
  className?: string;
}) {
  const [failed, setFailed] = useState(false);
  // A different image deserves its own chance to load.
  useEffect(() => setFailed(false), [imageUrl]);

  const showImage = !!imageUrl && !failed;
  return (
    <span
      className={`${size} shrink-0 rounded-md overflow-hidden flex items-center justify-center ${
        showImage ? 'bg-stone-100' : 'bg-stone-800 text-stone-400'
      } ${className}`}
    >
      {showImage ? (
        <img
          src={imageUrl}
          alt=""
          loading="lazy"
          // Don't tell every retailer CDN which loadout page the viewer is on. It also
          // gets past hotlink checks that only reject foreign referrers.
          referrerPolicy="no-referrer"
          onError={() => setFailed(true)}
          className="w-full h-full object-contain"
        />
      ) : (
        categoryIcon(category, iconSize)
      )}
    </span>
  );
}
